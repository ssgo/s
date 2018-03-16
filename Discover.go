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

var dcRedisService *redis.Redis
var dcRedisCalls *redis.Redis
var isService = false
var isClient = false
var syncerRunning = false
var syncerStopChan chan bool
var pingStopChan chan bool

//var forceDiscoverClient = false

var myAddr = ""
var appNodes = map[string]map[string]*NodeInfo{}

type NodeInfo struct {
	Addr        string
	Weight      int
	UsedTimes   uint64
	FailedTimes uint8
	Data        interface{}
}

var settedLoadBalancer LoadBalancer = &DefaultLoadBalancer{}
var appSubscribeKeys []interface{}
var appClientPools = map[string]*ClientPool{}

type Caller struct {
	request *http.Request
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
	if appNodes[app] == nil {
		log.Printf("DISCOVER	No App	%s	%s", app, path)
		return &Result{Error: fmt.Errorf("CALL	%s	%s	not exists", app, path)}, ""
	}
	if len(appNodes[app]) == 0 {
		log.Printf("DISCOVER	No Node	%s	%s	%d", app, path, len(appNodes[app]))
		return &Result{Error: fmt.Errorf("CALL	%s	%s	No node avaliable	(%d)", app, path, len(appNodes[app]))}, ""
	}

	appConf := config.Calls[app]
	if headers == nil {
		headers = []string{}
	}
	if appConf.AccessToken != "" {
		headers = append(headers, "Access-Token", appConf.AccessToken)
	}

	var r *Result
	excludes := make(map[string]bool)
	tryTimes := 0
	for {
		tryTimes++
		var node *NodeInfo
		if withNode != "" {
			node = appNodes[app][withNode]
			excludes[withNode] = true
			withNode = ""
		}

		if node == nil {
			nodes := make([]*NodeInfo, 0)
			for _, node := range appNodes[app] {
				if excludes[node.Addr] || node.FailedTimes >= config.CallRetryTimes {
					continue
				}
				nodes = append(nodes, node)
			}
			if len(nodes) > 0 {
				node = settedLoadBalancer.Next(nodes, caller.request)
				excludes[node.Addr] = true
			}
		}
		if node == nil {
			log.Printf("DISCOVER	No Node	%s	%s	%d / %d", app, path, tryTimes, len(appNodes[app]))
			break
		}

		// 请求节点
		startTime := time.Now()
		node.UsedTimes++
		r = appClientPools[app].DoByRequest(caller.request, method, fmt.Sprintf("http://%s%s", node.Addr, path), data, headers...)
		settedLoadBalancer.Response(node, r.Error, r.Response, startTime.UnixNano()-time.Now().UnixNano())

		if r.Error != nil || r.Response.StatusCode == 502 || r.Response.StatusCode == 503 || r.Response.StatusCode == 504 {
			statusCode := 0
			if r.Response != nil {
				statusCode = r.Response.StatusCode
			}
			log.Printf("DISCOVER	Failed	%s	%s	%d	%d	%d / %d	%d / %d	%d	%s", node.Addr, path, node.Weight, node.UsedTimes, tryTimes, len(appNodes[app]), node.FailedTimes, config.CallRetryTimes, statusCode, r.Error)
			// 错误处理
			node.FailedTimes++
			if node.FailedTimes >= config.CallRetryTimes {
				log.Printf("DISCOVER	Removed	%s	%s	%d	%d	%d / %d	%d / %d	%d	%s", node.Addr, path, node.Weight, node.UsedTimes, tryTimes, len(appNodes[app]), node.FailedTimes, config.CallRetryTimes, statusCode, r.Error)
				if dcRedisCalls.HDEL(config.RegistryPrefix+app, node.Addr) > 0 {
					dcRedisCalls.Do("PUBLISH", config.RegistryPrefix+"CH_"+config.App, fmt.Sprintf("%s %d", node.Addr, 0))
				}
			}
		} else {
			// 成功
			return r, node.Addr
		}
	}

	// 全部失败，返回最后一个失败的结果
	return &Result{Error: fmt.Errorf("CALL	%s	%s	No node avaliable	(%d / %d)", app, path, tryTimes, len(appNodes[app]))}, ""
}

func startDiscover(addr string) bool {
	myAddr = addr
	isService = config.App != "" && config.Weight > 0
	if isService {
		dcRedisService = redis.GetRedis(config.Registry)

		// 设置默认的AuthChecker
		if webAuthChecker == nil {
			SetAuthChecker(func(authLevel uint, url *string, in *map[string]interface{}, request *http.Request) bool {
				settedAuthLevel := config.AccessTokens[request.Header.Get("Access-Token")]
				//log.Println(" ***** ", request.Header.Get("Access-Token"), config.AccessTokens[request.Header.Get("Access-Token")], authLevel)
				return settedAuthLevel != nil && *settedAuthLevel >= authLevel
			})
		}

		// 注册节点
		if dcRedisService.HSET(config.RegistryPrefix+config.App, addr, config.Weight) {
			log.Printf("DISCOVER	Registered	%s	%s	%d", config.App, addr, config.Weight)
			dcRedisService.Do("PUBLISH", config.RegistryPrefix+"CH_"+config.App, fmt.Sprintf("%s %d", addr, config.Weight))
		} else {
			log.Printf("DISCOVER	Register failed	%s	%s	%d", config.App, addr, config.Weight)
			return false
		}
	}

	if len(config.Calls) > 0 {
		for app, conf := range config.Calls {
			addApp(app, *conf, false)
		}
		if RestartDiscoverSyncer() == false {
			return false
		}
	}
	return true
}

func AddExternalApp(app string, conf Call) bool {
	return addApp(app, conf, true)
}

func addApp(app string, conf Call, fetch bool) bool {
	if appClientPools[app] != nil {
		return false
	}
	if len(config.Calls) == 0 {
		config.Calls = make(map[string]*Call)
	}
	if config.Calls[app] == nil {
		config.Calls[app] = &conf
	}

	appNodes[app] = map[string]*NodeInfo{}
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

	// 立刻获取一次应用信息
	if fetch {
		fetchApp(app)
	}

	return true
}

var syncConn *redigo.PubSubConn

// 保持 redis 链接，否则会因为超时而发生错误
func pingRedis() {
	n := 15
	if dcRedisCalls.ReadTimeout > 2000 {
		n = dcRedisCalls.ReadTimeout / 1000 / 2
	} else if dcRedisCalls.ReadTimeout > 0 {
		n = 1
	}
	for {
		for i := 0; i < n; i++ {
			time.Sleep(time.Second * 1)
			if !syncerRunning {
				break
			}
		}
		if !syncerRunning {
			break
		}
		if syncConn != nil {
			syncConn.Ping("1")
		}
		if !syncerRunning {
			break
		}
	}
	if pingStopChan != nil {
		pingStopChan <- true
	}
}

func fetchApp(app string) {
	if dcRedisCalls == nil {
		dcRedisCalls = redis.GetRedis(config.RegistryCalls)
	}

	appResults := dcRedisCalls.Do("HGETALL", config.RegistryPrefix+app).ResultMap()
	for _, node := range appNodes[app] {
		if appResults[node.Addr] == nil {
			log.Printf("DISCOVER	Remove When Reset	%s	%s	%d", app, node.Addr, 0)
			pushNode(app, node.Addr, 0)
		}
	}
	for addr, weightResult := range appResults {
		weight := weightResult.Int()
		log.Printf("DISCOVER	Reset	%s	%s	%d", app, addr, weight)
		pushNode(app, addr, weight)
	}
}

func syncDiscover(initedChan chan bool) {
	inited := false
	for {
		syncConn = &redigo.PubSubConn{Conn: dcRedisCalls.GetConnection()}
		err := syncConn.Subscribe(appSubscribeKeys...)
		if err != nil {
			log.Print("REDIS SUBSCRIBE	", err)
			syncConn.Close()
			syncConn = nil

			if !inited {
				inited = true
				initedChan <- true
			}
			time.Sleep(time.Second * 1)
			if !syncerRunning {
				break
			}
			continue
		}

		// 第一次或断线后重新获取（订阅开始后再获取全量确保信息完整）
		for app := range config.Calls {
			fetchApp(app)
			//appResults := dcRedisCalls.Do("HGETALL", config.RegistryPrefix+app).ResultMap()
			//for _, node := range appNodes[app] {
			//	if appResults[node.Addr] == nil {
			//		log.Printf("DISCOVER	Remove When Reset	%s	%s	%d", app, node.Addr, 0)
			//		pushNode(app, node.Addr, 0)
			//	}
			//}
			//for addr, weightResult := range appResults {
			//	weight := weightResult.Int()
			//	log.Printf("DISCOVER	Reset	%s	%s	%d", app, addr, weight)
			//	pushNode(app, addr, weight)
			//}
		}
		if !inited {
			inited = true
			initedChan <- true
		}
		if !syncerRunning {
			break
		}

		// 开始接收订阅数据
		for {
			isErr := false
			switch v := syncConn.Receive().(type) {
			case redigo.Message:
				a := strings.Split(string(v.Data), " ")
				addr := a[0]
				weight := 0
				if len(a) == 2 {
					weight, _ = strconv.Atoi(a[1])
				}
				app := strings.Replace(v.Channel, config.RegistryPrefix+"CH_", "", 1)
				log.Printf("DISCOVER	Received	%s	%s	%d", app, addr, weight)
				pushNode(app, addr, weight)
			case redigo.Subscription:
			case redigo.Pong:
				//log.Print("	-0-0-0-0-0-0-	Pong")
			case error:
				if !strings.Contains(v.Error(), "connection closed") {
					log.Printf("REDIS RECEIVE ERROR	%s", v)
				}
				isErr = true
				break
			}
			if isErr {
				break
			}
			if !syncerRunning {
				break
			}
		}
		if !syncerRunning {
			break
		}
		time.Sleep(time.Second * 1)
		if !syncerRunning {
			break
		}
	}

	if syncConn != nil {
		syncConn.Unsubscribe(appSubscribeKeys)
		syncConn.Close()
		syncConn = nil
	}

	if syncerStopChan != nil {
		syncerStopChan <- true
	}
}

func pushNode(app, addr string, weight int) {
	if weight == 0 {
		// 删除节点
		if appNodes[app][addr] != nil {
			delete(appNodes[app], addr)
		}
	} else if appNodes[app][addr] == nil {
		// 新节点
		var avgScore float64 = 0
		for _, node := range appNodes[app] {
			avgScore = float64(node.UsedTimes) / float64(weight)
		}
		usedTimes := uint64(avgScore) * uint64(weight)
		appNodes[app][addr] = &NodeInfo{Addr: addr, Weight: weight, UsedTimes: usedTimes}
	} else if appNodes[app][addr].Weight != weight {
		// 修改权重
		node := appNodes[app][addr]
		node.Weight = weight
		node.UsedTimes = uint64(float64(node.UsedTimes) / float64(node.Weight) * float64(weight))
	}
}

func RestartDiscoverSyncer() bool {
	log.Print("DISCOVER	restarting	", appSubscribeKeys)
	if dcRedisCalls == nil {
		dcRedisCalls = redis.GetRedis(config.RegistryCalls)
	}

	if isClient == false {
		isClient = true
	}

	// 如果之前没有启动
	if syncConn != nil {
		log.Print("DISCOVER	stopping for restart")
		syncConn.Unsubscribe(appSubscribeKeys)
		syncConn.Close()
		syncConn = nil
		log.Print("DISCOVER	stopped for restart")
	}

	// 如果之前没有启动
	if syncerRunning == false {
		log.Print("DISCOVER	starting for restart")
		syncerRunning = true
		initedChan := make(chan bool)
		go syncDiscover(initedChan)
		<-initedChan
		go pingRedis()
		log.Print("DISCOVER	started for restart")
	}
	return true
}

func stopDiscover() {
	if isClient {
		syncerStopChan = make(chan bool)
		pingStopChan = make(chan bool)
		syncerRunning = false
		if syncConn != nil {
			log.Print("DISCOVER	unsubscribing	", appSubscribeKeys)
			syncConn.Unsubscribe(appSubscribeKeys)
			log.Print("DISCOVER	closeing syncConn")
			tmpConn := syncConn
			syncConn = nil
			go func() {
				tmpConn.Close()
				log.Print("DISCOVER	closed syncConn")
			}()
		}
	}

	if isService {
		if dcRedisService.HDEL(config.RegistryPrefix+config.App, myAddr) > 0 {
			log.Printf("DISCOVER	Unregistered	%s	%s	%d", config.App, myAddr, 0)
			dcRedisService.Do("PUBLISH", config.RegistryPrefix+"CH_"+config.App, fmt.Sprintf("%s %d", myAddr, 0))
		}
	}
}

func waitDiscover() {
	if isClient {
		if syncerStopChan != nil {
			<-syncerStopChan
			syncerStopChan = nil
		}
		if pingStopChan != nil {
			<-pingStopChan
			pingStopChan = nil
		}
	}
}
