package discover

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"github.com/ssgo/s/base"
	"github.com/ssgo/s/httpclient"
	"github.com/ssgo/s/redis"
)

var serverRedisPool *redis.Redis
var clientRedisPool *redis.Redis
var isServer = false
var isClient = false
var syncerRunning = false
var syncerStopChan chan bool
var pingStopChan chan bool

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
var appClientPools = map[string]*httpclient.ClientPool{}

func IsServer() bool {
	return isServer
}
func IsClient() bool {
	return isClient
}

func Start(addr string, conf Config) bool {
	myAddr = addr
	config = conf

	if config.CallTimeout <= 0 {
		config.CallTimeout = 5000
	}

	if config.Registry == "" {
		config.Registry = "127.0.0.1:6379:15"
	}
	if config.RegistryCalls == "" {
		config.RegistryCalls = "127.0.0.1:6379:15"
	}
	if config.CallRetryTimes <= 0 {
		config.CallRetryTimes = 10
	}

	if config.App != "" && config.App[0] == '_' {
		base.TraceLog("DC", map[string]interface{}{
			"type":  "startFailed",
			"error": "is a not available name",
			"app":   config.App,
		})
		//log.Print("ERROR	", config.App, " is a not available name")
		config.App = ""
	}

	if config.Weight <= 0 {
		config.Weight = 1
	}

	if config.XUniqueId == "" {
		config.XUniqueId = "X-Unique-Id"
	}
	if config.XForwardedForName == "" {
		config.XForwardedForName = "X-Forwarded-For"
	}
	if config.XRealIpName == "" {
		config.XRealIpName = "X-Real-Ip"
	}

	isServer = config.App != "" && config.Weight > 0
	if isServer {
		serverRedisPool = redis.GetRedis(config.Registry)

		// 注册节点
		if serverRedisPool.HSET(config.RegistryPrefix+config.App, addr, config.Weight) {
			base.Log("DC", map[string]interface{}{
				"type":   "registered",
				"app":    config.App,
				"addr":   addr,
				"weight": config.Weight,
			})
			//log.Printf("DISCOVER	Registered	%s	%s	%d", config.App, addr, config.Weight)
			serverRedisPool.Do("PUBLISH", config.RegistryPrefix+"CH_"+config.App, fmt.Sprintf("%s %d", addr, config.Weight))
		} else {
			base.TraceLog("DC", map[string]interface{}{
				"type":   "registerFailed",
				"app":    config.App,
				"addr":   addr,
				"weight": config.Weight,
			})
			//log.Printf("DISCOVER	Register failed	%s	%s	%d", config.App, addr, config.Weight)
			return false
		}
	}

	if len(config.Calls) > 0 {
		for app, conf := range config.Calls {
			addApp(app, *conf, false)
		}
		if Restart() == false {
			return false
		}
	}
	return true
}

func Restart() bool {
	base.Log("DC", map[string]interface{}{
		"type":             "restarting",
		"app":              config.App,
		"addr":             myAddr,
		"weight":           config.Weight,
		"appSubscribeKeys": appSubscribeKeys,
	})
	//log.Print("DISCOVER	restarting	", appSubscribeKeys)
	if clientRedisPool == nil {
		clientRedisPool = redis.GetRedis(config.RegistryCalls)
	}

	if isClient == false {
		isClient = true
	}

	// 如果之前没有启动
	if syncConn != nil {
		base.Log("DC", map[string]interface{}{
			"type":             "stoppingForRestart",
			"app":              config.App,
			"addr":             myAddr,
			"weight":           config.Weight,
			"appSubscribeKeys": appSubscribeKeys,
		})
		//log.Print("DISCOVER	stopping for restart")
		syncConn.Unsubscribe(appSubscribeKeys)
		syncConn.Close()
		syncConn = nil
		base.Log("DC", map[string]interface{}{
			"type":             "stoppedForRestart",
			"app":              config.App,
			"addr":             myAddr,
			"weight":           config.Weight,
			"appSubscribeKeys": appSubscribeKeys,
		})
		//log.Print("DISCOVER	stopped for restart")
	}

	// 如果之前没有启动
	if syncerRunning == false {
		base.Log("DC", map[string]interface{}{
			"type":             "startingForRestart",
			"app":              config.App,
			"addr":             myAddr,
			"weight":           config.Weight,
			"appSubscribeKeys": appSubscribeKeys,
		})
		//log.Print("DISCOVER	starting for restart")
		syncerRunning = true
		initedChan := make(chan bool)
		go syncDiscover(initedChan)
		<-initedChan
		go pingRedis()
		base.Log("DC", map[string]interface{}{
			"type":             "startedForRestart",
			"app":              config.App,
			"addr":             myAddr,
			"weight":           config.Weight,
			"appSubscribeKeys": appSubscribeKeys,
		})
		//log.Print("DISCOVER	started for restart")
	}
	return true
}

func Stop() {
	if isClient {
		syncerStopChan = make(chan bool)
		pingStopChan = make(chan bool)
		syncerRunning = false
		if syncConn != nil {
			base.Log("DC", map[string]interface{}{
				"type":             "unSubscribing",
				"app":              config.App,
				"addr":             myAddr,
				"weight":           config.Weight,
				"appSubscribeKeys": appSubscribeKeys,
			})
			//log.Print("DISCOVER	unsubscribing	", appSubscribeKeys)
			syncConn.Unsubscribe(appSubscribeKeys)
			base.Log("DC", map[string]interface{}{
				"type":             "closingSyncConn",
				"app":              config.App,
				"addr":             myAddr,
				"weight":           config.Weight,
				"appSubscribeKeys": appSubscribeKeys,
			})
			//log.Print("DISCOVER	closing syncConn")
			tmpConn := syncConn
			syncConn = nil
			go func() {
				tmpConn.Close()
				base.Log("DC", map[string]interface{}{
					"type":             "closedSyncConn",
					"app":              config.App,
					"addr":             myAddr,
					"weight":           config.Weight,
					"appSubscribeKeys": appSubscribeKeys,
				})
				//log.Print("DISCOVER	closed syncConn")
			}()
		}
	}

	if isServer {
		if serverRedisPool.HDEL(config.RegistryPrefix+config.App, myAddr) > 0 {
			base.Log("DC", map[string]interface{}{
				"type":             "unregistered",
				"app":              config.App,
				"addr":             myAddr,
				"weight":           config.Weight,
				"appSubscribeKeys": appSubscribeKeys,
			})
			//log.Printf("DISCOVER	Unregistered	%s	%s	%d", config.App, myAddr, 0)
			serverRedisPool.Do("PUBLISH", config.RegistryPrefix+"CH_"+config.App, fmt.Sprintf("%s %d", myAddr, 0))
		}
	}
}

func Wait() {
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

func AddExternalApp(app string, conf CallInfo) bool {
	return addApp(app, conf, true)
}

func addApp(app string, conf CallInfo, fetch bool) bool {
	if appClientPools[app] != nil {
		return false
	}
	if len(config.Calls) == 0 {
		config.Calls = make(map[string]*CallInfo)
	}
	if config.Calls[app] == nil {
		config.Calls[app] = &conf
	}

	appNodes[app] = map[string]*NodeInfo{}
	appSubscribeKeys = append(appSubscribeKeys, config.RegistryPrefix+"CH_"+app)

	timeout := conf.Timeout
	if timeout <= 0 {
		timeout = config.CallTimeout
	}
	var cp *httpclient.ClientPool
	if conf.HttpVersion == 1 {
		cp = httpclient.GetClient(time.Duration(timeout) * time.Millisecond)
	} else {
		cp = httpclient.GetClientH2C(time.Duration(timeout) * time.Millisecond)
	}
	cp.XUniqueId = config.XUniqueId
	cp.XRealIpName = config.XRealIpName
	cp.XForwardedForName = config.XForwardedForName
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
	if clientRedisPool.ReadTimeout > 2000 {
		n = clientRedisPool.ReadTimeout / 1000 / 2
	} else if clientRedisPool.ReadTimeout > 0 {
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
	if clientRedisPool == nil {
		clientRedisPool = redis.GetRedis(config.RegistryCalls)
	}

	appResults := clientRedisPool.Do("HGETALL", config.RegistryPrefix+app).ResultMap()
	for _, node := range appNodes[app] {
		if appResults[node.Addr] == nil {
			base.Log("DC", map[string]interface{}{
				"type":   "removeNode",
				"app":    app,
				"addr":   node.Addr,
				"weight": node.Weight,
				"node":   node,
				"nodes":  appNodes[app],
			})
			//log.Printf("DISCOVER	Remove When Reset	%s	%s	%d", app, node.Addr, 0)
			pushNode(app, node.Addr, 0)
		}
	}
	for addr, weightResult := range appResults {
		weight := weightResult.Int()
		base.Log("DC", map[string]interface{}{
			"type":   "resetNode",
			"app":    app,
			"addr":   addr,
			"weight": weight,
			"nodes":  appNodes[app],
		})
		//log.Printf("DISCOVER	Reset	%s	%s	%d", app, addr, weight)
		pushNode(app, addr, weight)
	}
}

func syncDiscover(initedChan chan bool) {
	inited := false
	for {
		syncConn = &redigo.PubSubConn{Conn: clientRedisPool.GetConnection()}
		err := syncConn.Subscribe(appSubscribeKeys...)
		if err != nil {
			base.Log("DC", map[string]interface{}{
				"type":             "subscribe",
				"appSubscribeKeys": appSubscribeKeys,
				"error":            err.Error(),
			})
			//log.Print("REDIS SUBSCRIBE	", err)
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
			//appResults := clientRedisPool.Do("HGETALL", config.RegistryPrefix+app).ResultMap()
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
				base.Log("DC", map[string]interface{}{
					"type":             "received",
					"app":              app,
					"weight":           weight,
					"nodes":            appNodes[app],
					"appSubscribeKeys": appSubscribeKeys,
				})
				//log.Printf("DISCOVER	Received	%s	%s	%d", app, addr, weight)
				pushNode(app, addr, weight)
			case redigo.Subscription:
			case redigo.Pong:
				//log.Print("	-0-0-0-0-0-0-	Pong")
			case error:
				if !strings.Contains(v.Error(), "connection closed") {
					base.Log("DC", map[string]interface{}{
						"type":             "receiveError",
						"appSubscribeKeys": appSubscribeKeys,
						"error":            v.Error(),
					})
					//log.Printf("REDIS RECEIVE ERROR	%s", v)
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
