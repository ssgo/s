package s

import (
	"fmt"
	redigo "github.com/garyburd/redigo/redis"
	"github.com/ssgo/redis"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var dcRedis *redis.Redis
var isService = false
var isClient = false
var syncerRunning = false
var syncerStopChan = make(chan bool)

//var dcAppVersions = make(map[string]uint64)
var myAddr = ""
var nodes = map[string]map[string]*nodeInfo{}

type nodeInfo struct {
	addr        string
	weight      int
	score       float64
	usedTimes   uint64
	failedTimes uint8
	flag        bool
}

var appSubscribeKeys []interface{}

var appClientPools = map[string]*ClientPool{}

type Caller struct {
	headers []string
}

func (caller *Caller) Get(app, path string, headers ...string) *Result {
	return caller.Do("GET", app, path, nil, headers...)
}
func (caller *Caller) Post(app, path string, data interface{}, headers ...string) *Result {
	return caller.Do("POST", app, path, data, headers...)
}
func (caller *Caller) Put(app, path string, data interface{}, headers ...string) *Result {
	return caller.Do("PUT", app, path, data, headers...)
}
func (caller *Caller) Delete(app, path string, data interface{}, headers ...string) *Result {
	return caller.Do("DELETE", app, path, data, headers...)
}
func (caller *Caller) Head(app, path string, data interface{}, headers ...string) *Result {
	return caller.Do("HEAD", app, path, data, headers...)
}
func (caller *Caller) Do(method, app, path string, data interface{}, headers ...string) *Result {
	r, _ := caller.DoWithNode(method, app, "", path, data, headers...)
	return r
}
func (caller *Caller) DoWithNode(method, app, withNode, path string, data interface{}, headers ...string) (*Result, string) {
	if nodes[app] == nil {
		return &Result{Error: fmt.Errorf("CALL	%s	%s	not exists", app, path)}, ""
	}
	//gotNodes := make(nodeList, 0)
	if len(nodes[app]) == 0 {
		return &Result{Error: fmt.Errorf("CALL	%s	%s	No node avaliable	(%d)", app, path, len(nodes[app]))}, ""
	}

	appConf := config.Calls[app]
	if headers == nil {
		headers = []string{}
	}
	if appConf.AccessToken != "" {
		headers = append(headers, "Access-Token", appConf.AccessToken)
	}
	headers = append(headers, caller.headers...)

	var r *Result
	excludes := make(map[string]bool)
	for {
		var node *nodeInfo
		if withNode != "" {
			node = nodes[app][withNode]
			excludes[withNode] = true
			withNode = ""
		}
		if node == nil {
			node = getNextNode(app, &excludes)
		}
		if node == nil {
			break
		}

		// 计算得分
		node.usedTimes++
		node.score = float64(node.usedTimes) / float64(node.weight)

		// 请求节点
		//t1 := time.Now()
		r = appClientPools[app].Do(method, fmt.Sprintf("http://%s%s", node.addr, path), data, headers...)
		//log.Print(" ==============	", app, path, "	", float32(time.Now().UnixNano()-t1.UnixNano()) / 1e6)

		if r.Error != nil || r.Response.StatusCode == 502 || r.Response.StatusCode == 503 || r.Response.StatusCode == 504 {
			// 错误处理
			node.failedTimes++
			if node.failedTimes >= 3 {
				fmt.Printf("DISCOVER	Removed	%s	%d	%d	%d	%d	%s\n", node.addr, node.weight, node.usedTimes, node.failedTimes, r.Response.StatusCode, r.Error)
				if dcRedis.HDEL(config.RegistryPrefix+app, node.addr) > 0 {
					dcRedis.Do("PUBLISH", config.RegistryPrefix+"CH_"+config.App, fmt.Sprintf("%s %d", node.addr, 0))
					//dcRedis.INCR(config.RegistryPrefix + "CH_" + app)
				}
			}
		} else {
			// 成功
			return r, node.addr
		}
	}

	// 全部失败，返回最后一个失败的结果
	return &Result{Error: fmt.Errorf("CALL	%s	%s	No node avaliable	(%d)", app, path, len(nodes[app]))}, ""
}

func getNextNode(app string, excludes *map[string]bool) *nodeInfo {
	var minScore float64 = -1
	var minNode *nodeInfo = nil
	for _, node := range nodes[app] {
		if node.failedTimes < 3 && (*excludes)[node.addr] == false && (minScore == -1 || node.score < minScore) {
			minScore = node.score
			minNode = node
		}
	}
	if minNode != nil {
		(*excludes)[minNode.addr] = true
	}
	return minNode
}

func startDiscover(addr string) bool {
	myAddr = addr
	isService = config.App != "" && config.Weight > 0
	isClient = len(config.Calls) > 0
	if isService || isClient {
		dcRedis = redis.GetRedis(config.Registry)
		if dcRedis.Error != nil {
			return false
		}
	} else {
		return true
	}

	isok := true

	if isService {
		// 设置默认的AuthChecker
		if webAuthChecker == nil {
			SetAuthChecker(func(authLevel uint, url *string, in *map[string]interface{}, request *http.Request) bool {
				//log.Println(" ***** ", (*headers)["AccessToken"], config.AccessTokens[(*headers)["AccessToken"]], authLevel)
				return config.AccessTokens[request.Header.Get("Access-Token")] >= authLevel
			})
		}

		// 注册节点
		if dcRedis.HSET(config.RegistryPrefix+config.App, addr, config.Weight) {
			log.Printf("DISCOVER	Registered	%s	%s	%d", config.App, addr, config.Weight)
			//dcRedis.INCR(config.RegistryPrefix +"VER_"+config.App)
			dcRedis.Do("PUBLISH", config.RegistryPrefix+"CH_"+config.App, fmt.Sprintf("%s %d", addr, config.Weight))
		} else {
			isok = false
			log.Printf("DISCOVER	Register failed	%s	%s	%d", config.App, addr, config.Weight)
		}
	}

	if isClient {
		syncerRunning = true
		for app, conf := range config.Calls {
			appSubscribeKeys = append(appSubscribeKeys, config.RegistryPrefix+"CH_"+app)

			var cp *ClientPool
			if conf.HttpVersion == 1 {
				cp = GetClient1()
			} else {
				cp = GetClient()
			}
			if conf.Timeout > 0 {
				cp.pool.Timeout = time.Duration(conf.Timeout) * time.Millisecond
			}
			appClientPools[app] = cp
		}
		go syncDiscover()
	}
	return isok
}

var syncConn *redigo.PubSubConn

func syncDiscover() {
	for {
		if !syncerRunning {
			break
		}

		syncConn = &redigo.PubSubConn{Conn: dcRedis.GetConnection()}
		err := syncConn.Subscribe(appSubscribeKeys...)
		if err != nil {
			log.Print("REDIS SUBSCRIBE	", err)
			syncConn.Close()
			syncConn = nil
			time.Sleep(time.Second * 1)
			continue
		}

		// 第一次或断线后重新获取（订阅开始后再获取全量确保信息完整）
		for app, _ := range config.Calls {
			weights := dcRedis.Do("HGETALL", config.RegistryPrefix+app).IntMap()
			nodes[app] = map[string]*nodeInfo{}
			for addr, weight := range weights {
				log.Printf("DISCOVER	Received	%s	%s	%d", app, addr, weight)
				nodes[app][addr] = &nodeInfo{addr: addr, weight: weight, score: 0, flag: true}
			}
		}
		if !syncerRunning {
			break
		}

		// 开始接收订阅数据
		for {
			isErr := false
			switch v := syncConn.Receive().(type) {
			case redigo.Message:
				fmt.Printf("%s: message: %s\n", v.Channel, v.Data)
				a := strings.Split(string(v.Data), " ")
				addr := a[0]
				weight := 0
				if len(a) == 2 {
					weight, _ = strconv.Atoi(a[1])
				}
				app := strings.Replace(v.Channel, config.RegistryPrefix+"CH_", "", 1)
				log.Printf("DISCOVER	Received	%s	%s	%d", app, addr, weight)
				if weight == 0 {
					delete(nodes[app], addr)
				} else {
					nodes[app][addr] = &nodeInfo{addr: addr, weight: weight, score: 0, flag: true}
				}
			case redigo.Subscription:
			case error:
				if !strings.Contains(v.Error(), "connection closed") {
					fmt.Printf("REDIS RECEIVE ERROR	%s\n", v)
				}
				isErr = true
				break
			}
			if isErr {
				break
			}
		}
		if !syncerRunning {
			break
		}
		time.Sleep(time.Second * 1)
	}

	if syncConn != nil {
		syncConn.Unsubscribe(appSubscribeKeys)
		syncConn.Close()
		syncConn = nil
	}

	syncerStopChan <- true
}

//func syncDiscoverOld() {
//	for {
//		for i := 0; i < 3; i++ {
//			time.Sleep(time.Millisecond * 500)
//			if !syncerRunning {
//				break
//			}
//		}
//
//		remoteAppVersions := map[string]uint64{}
//		dcRedis.Do("MGET", appSubscribeKeys...).To(&remoteAppVersions)
//		for verKey, remoteAppVersion := range remoteAppVersions {
//			app := strings.Replace(verKey, config.RegistryPrefix+"VER_", "", 1)
//			if remoteAppVersion > dcAppVersions[app] {
//				dcAppVersions[app] = remoteAppVersion
//
//				// 获取该应用的节点
//				weights := dcRedis.Do("HGETALL", config.RegistryPrefix+app).IntMap()
//
//				// 第一个节点，初始化
//				if nodes[app] == nil {
//					nodes[app] = map[string]*nodeInfo{}
//				}
//
//				// 标记旧节点，得到平均分
//				var avgScore float64 = 0
//				for _, node := range nodes[app] {
//					node.flag = false
//					if avgScore == 0 && node.score > 0 {
//						avgScore = node.score
//					}
//				}
//
//				// 合并数据
//				for addr, weight := range weights {
//					if nodes[app][addr] == nil {
//						// 新节点使用平均分进行初始化
//						nodes[app][addr] = &nodeInfo{addr: addr, weight: weight, score: avgScore, usedTimes: uint64(avgScore) * uint64(weight), flag: true}
//					} else {
//						node := nodes[app][addr]
//						if weight != node.weight {
//							node.weight = weight
//							// 旧节点重新计算得分
//							node.usedTimes = uint64(avgScore) * uint64(weight)
//						}
//						node.flag = true
//					}
//				}
//
//				// 删除已经不存在了的节点
//				for addr, node := range nodes[app] {
//					if node.flag == false {
//						delete(nodes[app], addr)
//					}
//				}
//			}
//		}
//
//		if !syncerRunning {
//			break
//		}
//	}
//	syncerStopChan <- true
//}

func stopDiscover() {
	if isClient {
		syncerRunning = false
		if syncConn != nil {
			syncConn.Unsubscribe(appSubscribeKeys)
			syncConn.Close()
			syncConn = nil
		}
	}

	if isService {
		if dcRedis.HDEL(config.RegistryPrefix+config.App, myAddr) > 0 {
			log.Printf("DISCOVER	Unregistered	%s	%s	%d", config.App, myAddr, 0)
			dcRedis.Do("PUBLISH", config.RegistryPrefix+"CH_"+config.App, fmt.Sprintf("%s	%d", myAddr, 0))
			//dcRedis.INCR(config.RegistryPrefix +"VER_"+config.App)
		}
	}
}

func waitDiscover() {
	if isClient {
		<-syncerStopChan
	}
}
