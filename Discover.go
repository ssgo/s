package s

import (
	"fmt"
	"github.com/ssgo/redis"
	"log"
	"strings"
	"time"
	"net/http"
)

var dcRedis *redis.Redis
var isService = false
var isClient = false
var syncerRunning = false
var syncerStopChan = make(chan bool)
var dcAppVersions = make(map[string]uint64)
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

var appVersionsKeys []interface{}

var appClientPools = map[string]*ClientPool{}

type Caller struct {
	headers []string
}

// TODO CallWithNode
//func (caller *Caller) CallWithNode(addr string, app, path string, data interface{}, headers ... string) *Result{
//
//}

func (caller *Caller) Get(app, path string, headers ... string) *Result {
	return caller.Post(app, path, nil, headers...)
}
func (caller *Caller) Post(app, path string, data interface{}, headers ... string) *Result {
	if nodes[app] == nil {
		return &Result{Error: fmt.Errorf("CALL	%s	%s	not exists", app, path)}
	}
	//gotNodes := make(nodeList, 0)
	if len(nodes[app]) == 0 {
		return &Result{Error: fmt.Errorf("CALL	%s	%s	No node avaliable	(%d)", app, path, len(nodes[app]))}
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
		node := getNextNode(app, &excludes)
		if node == nil {
			break
		}

		// 计算得分
		node.usedTimes++
		node.score = float64(node.usedTimes) / float64(node.weight)

		// 请求节点
		//t1 := time.Now()
		r = appClientPools[app].Do(fmt.Sprintf("http://%s%s", node.addr, path), data, headers...)
		//log.Print(" ==============	", app, path, "	", float32(time.Now().UnixNano()-t1.UnixNano()) / 1e6)

		if r.Error != nil || r.Response.StatusCode == 502 || r.Response.StatusCode == 503 || r.Response.StatusCode == 504 {
			// 错误处理
			node.failedTimes++
			if node.failedTimes >= 3 {
				fmt.Printf("DC	REMOVE	%s	%d	%d	%d	%d	%s\n", node.addr, node.weight, node.usedTimes, node.failedTimes, r.Response.StatusCode, r.Error)
				if dcRedis.HDEL(config.DiscoverPrefix+app, node.addr) > 0 {
					dcRedis.INCR(config.DiscoverPrefix+"VER_"+app)
				}
			}
		} else {
			// 成功
			return r
		}
	}

	// 全部失败，返回最后一个失败的结果
	return &Result{Error: fmt.Errorf("CALL	%s	%s	No node avaliable	(%d)", app, path, len(nodes[app]))}
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
		dcRedis = redis.GetRedis(config.Discover)
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
			SetWebAuthChecker(func(authLevel uint, url *string, in *map[string]interface{}, request *http.Request) bool {
				//log.Println(" ***** ", (*headers)["AccessToken"], config.AccessTokens[(*headers)["AccessToken"]], authLevel)
				return config.AccessTokens[request.Header.Get("Access-Token")] >= authLevel
			})
		}

		// 注册节点
		if dcRedis.HSET(config.DiscoverPrefix+config.App, addr, config.Weight) {
			dcRedis.INCR(config.DiscoverPrefix+"VER_"+config.App)
		} else {
			isok = false
			log.Println("DISCOVER	Register failed", config.App, addr, config.Weight)
		}
	}

	if isClient {
		syncerRunning = true
		for app, conf := range config.Calls {
			weights := dcRedis.Do("HGETALL", config.DiscoverPrefix+app).IntMap()
			nodes[app] = map[string]*nodeInfo{}
			for addr, weight := range weights {
				nodes[app][addr] = &nodeInfo{addr: addr, weight: weight, score: 0, flag: true}
			}
			dcAppVersions[app] = dcRedis.GET(config.DiscoverPrefix + "VER_" + app).Uint64()
			appVersionsKeys = append(appVersionsKeys, config.DiscoverPrefix+"VER_"+app)

			var cp *ClientPool
			if conf.HttpVersion == 1{
				cp = GetClient1()
			}else{
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

func syncDiscover() {
	for {
		for i := 0; i < 3; i++ {
			time.Sleep(time.Millisecond * 500)
			if !syncerRunning {
				break
			}
		}

		remoteAppVersions := map[string]uint64{}
		dcRedis.Do("MGET", appVersionsKeys...).To(&remoteAppVersions)
		for verKey, remoteAppVersion := range remoteAppVersions {
			app := strings.Replace(verKey, config.DiscoverPrefix+"VER_", "", 1)
			if remoteAppVersion > dcAppVersions[app] {
				dcAppVersions[app] = remoteAppVersion

				// 获取该应用的节点
				weights := dcRedis.Do("HGETALL", config.DiscoverPrefix+app).IntMap()

				// 第一个节点，初始化
				if nodes[app] == nil {
					nodes[app] = map[string]*nodeInfo{}
				}

				// 标记旧节点，得到平均分
				var avgScore float64 = 0
				for _, node := range nodes[app] {
					node.flag = false
					if avgScore == 0 && node.score > 0 {
						avgScore = node.score
					}
				}

				// 合并数据
				for addr, weight := range weights {
					if nodes[app][addr] == nil {
						// 新节点使用平均分进行初始化
						nodes[app][addr] = &nodeInfo{addr: addr, weight: weight, score: avgScore, usedTimes: uint64(avgScore) * uint64(weight), flag: true}
					} else {
						node := nodes[app][addr]
						if weight != node.weight {
							node.weight = weight
							// 旧节点重新计算得分
							node.usedTimes = uint64(avgScore) * uint64(weight)
						}
						node.flag = true
					}
				}

				// 删除已经不存在了的节点
				for addr, node := range nodes[app] {
					if node.flag == false {
						delete(nodes[app], addr)
					}
				}
			}
		}

		if !syncerRunning {
			break
		}
	}
	syncerStopChan <- true
}

func stopDiscover() {
	if isClient {
		syncerRunning = false
	}

	if isService {
		if dcRedis.HDEL(config.DiscoverPrefix+config.App, myAddr) > 0 {
			dcRedis.INCR(config.DiscoverPrefix+"VER_"+config.App)
		}
	}
}

func waitDiscover() {
	if isClient {
		<-syncerStopChan
	}
}
