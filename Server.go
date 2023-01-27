package s

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/ssgo/config"
	"github.com/ssgo/discover"
	"github.com/ssgo/httpclient"
	"github.com/ssgo/log"
	"github.com/ssgo/redis"
	"github.com/ssgo/standard"
	"github.com/ssgo/u"
	"golang.org/x/net/http2"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Arr = []interface{}

type Map = map[string]interface{}

var name = "Noname Server"
var version = "unset version"
var inited = false
var running = false

type serviceConfig struct {
	Listen string
	SSL    map[string]*CertSet
	//HttpVersion                   int
	KeepaliveTimeout              int
	NoLogGets                     bool
	NoLogHeaders                  string
	NoLogInputFields              bool
	LogInputArrayNum              int
	LogInputFieldSize             int
	NoLogOutputFields             string
	LogOutputArrayNum             int
	LogOutputFieldSize            int
	LogWebsocketAction            bool
	Compress                      bool
	CompressMinSize               int
	CompressMaxSize               int
	CertFile                      string
	KeyFile                       string
	CheckDomain                   string // 心跳检测时使用域名
	AccessTokens                  map[string]*int
	RewriteTimeout                int
	AcceptXRealIpWithoutRequestId bool
	StatisticTime                 bool
	StatisticTimeInterval         int
	Fast                          bool
	MaxUploadSize                 int64
	IpPrefix                      string // 指定使用的IP网段，默认排除 172.17
	Cpu                           int    // CPU占用的核数，默认为0，即不做限制
	Memory                        int    // 内存（单位M），默认为0，即不做限制
	CpuMonitor                    bool
	MemoryMonitor                 bool
	CookieScope                   string // Cookie的有效范围，host|domain|topDomain，默认值为host
	IdServer                      string // 用来维护唯一ID的redis服务器连接
}

type CertSet struct {
	CertFile string
	KeyFile  string
}

type Argot string

type Result struct {
	Ok      bool
	Argot   Argot
	Message string
}

type CodeResult struct {
	Code    int
	Message string
}

var _argots = make([]ArgotInfo, 0)

var Config = serviceConfig{}

var accessTokens = map[string]*int{}

var _cpuOverTimes uint = 0
var _memoryOutTimes uint = 0

func GetCPUMemoryStat() (uint, uint) {
	return _cpuOverTimes, _memoryOutTimes
}

//var callTokens = map[string]*string{}

//type Call struct {
//	AccessToken string
//	Host        string
//	Timeout     int
//	HttpVersion int
//	WithSSL     bool
//}

var _rd *redis.Redis
var _rd2 *redis.Redis
var _rdStarted bool

func SetName(serverName string) {
	name = serverName
}

func SetVersion(serverVersion string) {
	version = serverVersion
}

func getRedis() *redis.Redis {
	if _rd == nil {
		_rd = redis.GetRedis(discover.Config.Registry, serverLogger)
	}
	return _rd
}

func getPubSubRedis() *redis.Redis {
	if _rd2 == nil {
		rd := getRedis()
		confForPubSub := *rd.Config
		confForPubSub.IdleTimeout = -1
		confForPubSub.ReadTimeout = -1
		_rd2 = redis.NewRedis(&confForPubSub, serverLogger)
	}
	return _rd2
}

var noLogHeaders = map[string]bool{}

//var encryptLogFields = map[string]bool{}
var noLogOutputFields = map[string]bool{}

var serverId = u.UniqueId()
var serverStartTime = time.Now()
var serverLogger *log.Logger

var serverAddr string
var serverProto = "http"
var serverProtoName = "http"
var checker func(request *http.Request) bool

var shutdownHooks = make([]func(), 0)

func AddShutdownHook(f func()) {
	shutdownHooks = append(shutdownHooks, f)
}

type timerServer struct {
	name             string
	intervalDuration time.Duration
	intervalTimes    int
	running          bool
	stopChan         chan bool
	run              func(*bool)
	start            func()
	stop             func()
}

var timerServers = make([]*timerServer, 0)

func NewTimerServer(name string, interval time.Duration, run func(*bool), start func(), stop func()) {
	if interval < time.Millisecond*500 {
		interval = time.Millisecond * 500
	}
	intervalTimes := int(interval / (time.Millisecond * 500))
	timerServers = append(timerServers, &timerServer{name: name, intervalDuration: interval, intervalTimes: intervalTimes, run: run, start: start, stop: stop})
}

//var initFunc func()
//var startFunc func() bool
//var stopFunc func()
//var waitFunc func()
//
//func OnInit(f func()) {
//	initFunc = f
//}
//
//func OnStart(f func() bool) {
//	startFunc = f
//}
//
//func OnStop(f func()) {
//	stopFunc = f
//}
//
//func OnWait(f func()) {
//	waitFunc = f
//}

func logInfo(info string, extra ...interface{}) {
	serverLogger.Server(info, discover.Config.App, discover.Config.Weight, serverAddr, serverProto, serverStartTime, extra...)
}

func logError(error string, extra ...interface{}) {
	serverLogger.ServerError(error, discover.Config.App, discover.Config.Weight, serverAddr, serverProto, serverStartTime, extra...)
}

func SetChecker(ck func(request *http.Request) bool) {
	checker = ck
}

func GetServerAddr() string {
	return serverAddr
}

//noinspection GoUnusedParameter
func DefaultAuthChecker(authLevel int, logger *log.Logger, url *string, in map[string]interface{}, request *http.Request, response *Response, options *WebServiceOptions) (pass bool, sessionObject interface{}) {
	if authLevel == 0 {
		return true, nil
	}
	setAuthLevel := accessTokens[request.Header.Get("Access-Token")]
	return setAuthLevel != nil && *setAuthLevel >= authLevel, nil
}

func defaultChecker(request *http.Request, response http.ResponseWriter) {
	pid := request.Header.Get("Pid")
	if pid != "" && pid != strconv.Itoa(serviceInfo.pid) {
		response.WriteHeader(ResponseCodeHeartbeatPidError)
		return
	}

	var ok bool
	if checker != nil {
		ok = running && checker(request)
	} else {
		ok = running
	}

	cpuOverTimes, memoryOutTimes := GetCPUMemoryStat()
	if ok && Config.CpuMonitor && cpuOverTimes >= 10 {
		// CPU连续10分钟达到 100%
		ok = false
	}
	if ok && Config.MemoryMonitor && memoryOutTimes >= 10 {
		// 内存连续10分钟达到 100%
		ok = false
	}

	if ok {
		response.WriteHeader(ResponseCodeHeartbeatSucceed)
	} else {
		if !running {
			response.WriteHeader(ResponseCodeServiceNotRunning)
		} else {
			response.WriteHeader(ResponseCodeHeartbeatFailed)
		}
	}
}

//// 启动HTTP/1.1服务
//func Start1() {
//	start(1, nil)
//}
//
//// 启动HTTP/2服务
//func Start() {
//	start(2, nil)
//}

type Listen struct {
	listener net.Listener
	addr     string
	protocol string
	//httpVersion int
	//certFile    string
	//keyFile     string
}

type AsyncServer struct {
	startChan    chan bool
	stopChan     chan bool
	listens      []Listen
	Addr         string
	clientPool   *httpclient.ClientPool
	routeHandler routeHandler
	waited       bool
}

func (as *AsyncServer) Wait() {
	if !as.waited {
		for i := len(as.listens) - 1; i >= 0; i-- {
			<-as.stopChan
		}

		// 停止发现服务
		if discover.IsClient() || discover.IsServer() {
			logInfo("stopping discover")
			discover.Stop()
		}
		logInfo("stopping router")
		as.routeHandler.Stop()

		// 停止计时器服务
		for _, ts := range timerServers {
			logInfo("stopping timer server", "name", ts.name, "interval", ts.intervalDuration)
			if ts.stop != nil {
				ts.stop()
			}
			ts.running = false
		}

		// 停止Redis推送服务
		if _rd != nil && _rdStarted {
			_rd.Stop()
		}

		as.waited = true
		logInfo("waiting router")
		as.routeHandler.Wait()
		logInfo("waiting discover")
		discover.Wait()

		for _, ts := range timerServers {
			logInfo("waiting timer server", "name", ts.name, "interval", ts.intervalDuration)
			if ts.stopChan != nil {
				<-ts.stopChan
				ts.stopChan = nil
			}
		}

		if len(shutdownHooks) > 0 {
			for _, f := range shutdownHooks {
				f()
			}
			logInfo("shutdownHooks stopped", "shutdownHooksNum", len(shutdownHooks))
		}

		serviceInfo.remove()
		logInfo("stopped")

		// 最后关闭日志服务
		logInfo("logger stopped")
		log.Stop()
		log.Wait()

	}
}

func (as *AsyncServer) stop() {
	for _, listen := range as.listens {
		_ = listen.listener.Close()
	}
	running = false
}

func (as *AsyncServer) Stop() {
	as.stop()
	as.Wait()
}

func (as *AsyncServer) Get(path string, headers ...string) *httpclient.Result {
	return as.Do("GET", path, nil, headers...)
}
func (as *AsyncServer) Post(path string, data interface{}, headers ...string) *httpclient.Result {
	return as.Do("POST", path, data, headers...)
}
func (as *AsyncServer) Put(path string, data interface{}, headers ...string) *httpclient.Result {
	return as.Do("PUT", path, data, headers...)
}
func (as *AsyncServer) Delete(path string, data interface{}, headers ...string) *httpclient.Result {
	return as.Do("DELETE", path, data, headers...)
}
func (as *AsyncServer) Head(path string, data interface{}, headers ...string) *httpclient.Result {
	return as.Do("HEAD", path, data, headers...)
}
func (as *AsyncServer) Do(method, path string, data interface{}, headers ...string) *httpclient.Result {
	//r := as.clientPool.Do(method, fmt.Sprintf("%s://%s%s", u.StringIf(as.listens[0].certFile != "" && as.listens[0].keyFile != "", "https", "http"), as.Addr, path), data, headers...)
	r := as.clientPool.Do(method, fmt.Sprintf("%s://%s%s", serverProtoName, as.Addr, path), data, headers...)
	//if usedSessionIdKey != "" && r.Response != nil && r.Response.Header != nil && r.Response.Header.Get(usedSessionIdKey) != "" {
	//	as.clientPool.SetGlobalHeader(usedSessionIdKey, r.Response.Header.Get(usedSessionIdKey))
	//}
	return r
}

func (as *AsyncServer) SetGlobalHeader(k, v string) {
	as.clientPool.SetGlobalHeader(k, v)
}

func (as *AsyncServer) NewClient(timeout time.Duration) *Client {
	c := &Client{addr:as.Addr}
	if as.listens[0].protocol != "h2c" {
		c.clientPool = httpclient.GetClient(timeout)
	} else {
		c.clientPool = httpclient.GetClientH2C(timeout)
	}
	return c
}

type Client struct {
	addr string
	clientPool *httpclient.ClientPool
}

func (c *Client) Get(path string, headers ...string) *httpclient.Result {
	return c.Do("GET", path, nil, headers...)
}

func (c *Client) Post(path string, data interface{}, headers ...string) *httpclient.Result {
	return c.Do("POST", path, data, headers...)
}

func (c *Client) Put(path string, data interface{}, headers ...string) *httpclient.Result {
	return c.Do("PUT", path, data, headers...)
}

func (c *Client) Delete(path string, data interface{}, headers ...string) *httpclient.Result {
	return c.Do("DELETE", path, data, headers...)
}

func (c *Client) Head(path string, data interface{}, headers ...string) *httpclient.Result {
	return c.Do("HEAD", path, data, headers...)
}

func (c *Client) Do(method, path string, data interface{}, headers ...string) *httpclient.Result {
	r := c.clientPool.Do(method, fmt.Sprintf("%s://%s%s", serverProtoName, c.addr, path), data, headers...)
	return r
}

func AsyncStart() *AsyncServer {
	as := &AsyncServer{startChan: make(chan bool, 1)}
	as.Start()
	//start(as)
	<-as.startChan
	//if Config.HttpVersion == 1 || Config.CertFile != "" {
	if as.listens[0].protocol != "h2c" {
		as.clientPool = httpclient.GetClient(time.Duration(Config.RewriteTimeout) * time.Millisecond)
	} else {
		as.clientPool = httpclient.GetClientH2C(time.Duration(Config.RewriteTimeout) * time.Millisecond)
	}
	return as
}

func Init() {
	inited = true
	config.LoadConfig("service", &Config)

	accessTokens = Config.AccessTokens
	Config.AccessTokens = nil

	if Config.KeepaliveTimeout <= 0 {
		Config.KeepaliveTimeout = 15000
	}

	if Config.CompressMinSize <= 0 {
		Config.CompressMinSize = 1024
	}

	if Config.CompressMaxSize <= 0 {
		Config.CompressMaxSize = 4096000
	}

	if Config.RewriteTimeout <= 0 {
		Config.RewriteTimeout = 10000
	}

	if Config.NoLogHeaders == "" {
		Config.NoLogHeaders = fmt.Sprint("Accept,Accept-Encoding,Cache-Control,Pragma,Connection,Upgrade-Insecure-Requests")
	}
	for _, k := range strings.Split(strings.ToLower(Config.NoLogHeaders), ",") {
		noLogHeaders[strings.TrimSpace(k)] = true
	}

	if usedDeviceIdKey != "" {
		noLogHeaders[strings.ToLower(usedDeviceIdKey)] = true
	}
	if usedClientAppKey != "" {
		noLogHeaders[strings.ToLower(usedClientAppKey+"Name")] = true
		noLogHeaders[strings.ToLower(usedClientAppKey+"Version")] = true
	}
	if usedSessionIdKey != "" {
		noLogHeaders[strings.ToLower(usedSessionIdKey)] = true
	}

	noLogHeaders[strings.ToLower(standard.DiscoverHeaderClientIp)] = true
	noLogHeaders[strings.ToLower(standard.DiscoverHeaderForwardedFor)] = true
	noLogHeaders[strings.ToLower(standard.DiscoverHeaderUserId)] = true
	noLogHeaders[strings.ToLower(standard.DiscoverHeaderDeviceId)] = true
	noLogHeaders[strings.ToLower(standard.DiscoverHeaderClientAppName)] = true
	noLogHeaders[strings.ToLower(standard.DiscoverHeaderClientAppVersion)] = true
	noLogHeaders[strings.ToLower(standard.DiscoverHeaderSessionId)] = true
	noLogHeaders[strings.ToLower(standard.DiscoverHeaderRequestId)] = true
	noLogHeaders[strings.ToLower(standard.DiscoverHeaderHost)] = true
	noLogHeaders[strings.ToLower(standard.DiscoverHeaderScheme)] = true
	noLogHeaders[strings.ToLower(standard.DiscoverHeaderFromApp)] = true
	noLogHeaders[strings.ToLower(standard.DiscoverHeaderFromNode)] = true
	noLogHeaders[strings.ToLower(standard.DiscoverHeaderUserAgent)] = true

	if Config.NoLogOutputFields == "" {
		Config.NoLogOutputFields = ""
	}
	for _, k := range strings.Split(strings.ToLower(Config.NoLogOutputFields), ",") {
		noLogOutputFields[strings.TrimSpace(k)] = true
	}

	if Config.LogInputArrayNum <= 0 {
		Config.LogInputArrayNum = 10
	}

	if Config.LogOutputArrayNum <= 0 {
		Config.LogOutputArrayNum = 3
	}

	if Config.LogInputFieldSize <= 0 {
		Config.LogInputFieldSize = 500
	}

	if Config.LogOutputFieldSize <= 0 {
		Config.LogOutputFieldSize = 100
	}

	if Config.MaxUploadSize <= 0 {
		Config.MaxUploadSize = 1024 * 1024 * 10
	}

	if Config.Listen == "" {
		Config.Listen = ":"
	}

	serverAddr = Config.Listen

	if Config.CookieScope == "" {
		Config.CookieScope = "host"
	}

	// CPU和内存限制
	if Config.Cpu > 0 {
		if runtime.NumCPU() < Config.Cpu {
			Config.Cpu = runtime.NumCPU()
		}
		runtime.GOMAXPROCS(Config.Cpu)
	} else {
		Config.Cpu = runtime.NumCPU()
	}

	if Config.Memory <= 0 {
		if memoryInfo, err := mem.VirtualMemory(); err == nil {
			Config.Memory = int(memoryInfo.Total / 1024 / 1024)
		} else {
			Config.Memory = 8192
		}
	}
	if Config.Memory < 256 {
		Config.Memory = 256
	}
	//ms := runtime.MemStats{}
	//runtime.ReadMemStats(&ms)

	if Config.CpuMonitor || Config.MemoryMonitor {
		var serviceProcess *process.Process
		var cpuCounter *Counter
		var memoryCounter *Counter
		var memoryStat = runtime.MemStats{}
		NewTimerServer("serverMonitor", time.Minute, func(serverRunning *bool) {
			if Config.MemoryMonitor {
				runtime.ReadMemStats(&memoryStat)
				memoryUsed := byteToM(memoryStat.Sys)
				memoryPercent := math.Round(float64(memoryUsed)/float64(Config.Memory)*10000) / 100
				if memoryPercent >= 60 {
					memoryCounter.AddFailed(memoryPercent)
				} else {
					memoryCounter.Add(memoryPercent)
				}

				if memoryPercent >= 100 {
					_memoryOutTimes++
				} else {
					_memoryOutTimes = 0
				}

				if memoryCounter.Times >= 10 {
					memoryCounter.Count()
					memoryInfo := []interface{}{"memoryUsed", memoryUsed, "limit", Config.Memory, "memoryPercent", memoryPercent, "heapInuse", byteToM(memoryStat.HeapInuse), "heapIdle", byteToM(memoryStat.HeapIdle), "stackInuse", byteToM(memoryStat.StackInuse), "heapInuse", byteToM(memoryStat.HeapInuse), "pauseTotalNs", memoryStat.PauseTotalNs, "numGC", memoryStat.NumGC, "numForcedGC", memoryStat.NumForcedGC}
					if memoryPercent >= 100 {
						logError("out of memory", memoryInfo...)
					} else if memoryPercent >= 80 {
						logError("memory danger", memoryInfo...)
					} else if memoryPercent >= 60 {
						logError("memory warning", memoryInfo...)
					}
					serverLogger.Statistic(serverId, discover.Config.App, "memoryCount", memoryCounter.StartTime, memoryCounter.EndTime, memoryCounter.Times, memoryCounter.Failed, memoryCounter.Avg, memoryCounter.Min, memoryCounter.Max, memoryInfo...)
					memoryCounter.Reset()
				}
			}

			if Config.CpuMonitor && serviceProcess != nil {
				if cpuPercent, err := serviceProcess.CPUPercent(); err == nil {
					if cpuPercent >= 60 {
						cpuCounter.AddFailed(cpuPercent)
					} else {
						cpuCounter.Add(cpuPercent)
					}

					if cpuPercent >= 100 {
						_cpuOverTimes++
					} else {
						_cpuOverTimes = 0
					}

					if cpuCounter.Times >= 10 {
						cpuCounter.Count()
						numThreads, _ := serviceProcess.NumThreads()
						cpuInfo := []interface{}{"threads", numThreads, "goroutine", runtime.NumGoroutine(), "limit", Config.Cpu}
						if cpuPercent >= 100 {
							logError("over load of cpu", cpuInfo...)
						} else if cpuPercent >= 80 {
							logError("cpu danger", cpuInfo...)
						} else if cpuPercent >= 60 {
							logError("cpu warning", cpuInfo...)
						}
						serverLogger.Statistic(serverId, discover.Config.App, "cpuCount", cpuCounter.StartTime, cpuCounter.EndTime, cpuCounter.Times, cpuCounter.Failed, cpuCounter.Avg, cpuCounter.Min, cpuCounter.Max, cpuInfo...)
						cpuCounter.Reset()
					}
				}
			}
		}, func() {
			if Config.MemoryMonitor {
				memoryCounter = NewCounter()
			}

			if Config.CpuMonitor {
				var err error
				serviceProcess, err = process.NewProcess(int32(os.Getpid()))
				if err != nil {
					logError(err.Error())
					serviceProcess = nil
				}
				cpuCounter = NewCounter()
			}
		}, func() {
			if Config.MemoryMonitor {
				if memoryCounter.Times >= 1 {
					memoryUsed := byteToM(memoryStat.Sys)
					memoryPercent := math.Round(float64(memoryUsed)/float64(Config.Memory)*10000) / 100
					memoryInfo := []interface{}{"memoryUsed", memoryUsed, "limit", Config.Memory, "memoryPercent", memoryPercent, "heapInuse", byteToM(memoryStat.HeapInuse), "heapIdle", byteToM(memoryStat.HeapIdle), "stackInuse", byteToM(memoryStat.StackInuse), "heapInuse", byteToM(memoryStat.HeapInuse), "pauseTotalNs", memoryStat.PauseTotalNs, "numGC", memoryStat.NumGC, "numForcedGC", memoryStat.NumForcedGC}
					serverLogger.Statistic(serverId, discover.Config.App, "memoryCount", memoryCounter.StartTime, memoryCounter.EndTime, memoryCounter.Times, memoryCounter.Failed, memoryCounter.Avg, memoryCounter.Min, memoryCounter.Max, memoryInfo...)
					memoryCounter.Reset()
				}
			}

			if Config.CpuMonitor && serviceProcess != nil {
				if cpuCounter.Times >= 1 {
					numThreads, _ := serviceProcess.NumThreads()
					cpuInfo := []interface{}{"threads", numThreads, "goroutine", runtime.NumGoroutine(), "limit", Config.Cpu}
					serverLogger.Statistic(serverId, discover.Config.App, "cpuCount", cpuCounter.StartTime, cpuCounter.EndTime, cpuCounter.Times, cpuCounter.Failed, cpuCounter.Avg, cpuCounter.Min, cpuCounter.Max, cpuInfo...)
					cpuCounter.Reset()
				}
			}
		})
	}
}

func byteToM(n uint64) int {
	return int(math.Round(float64(n) / 1024 / 1024))
}

var timeStatistic *TimeStatistician

func Start() {
	//start(nil)
	AsyncStart().Wait()
}

func (as *AsyncServer) Start() {
	serverLogger = log.New(serverId)
	CheckCmd()

	log.Start()
	logInfo("logger started")

	// document must after registers
	//if inDocumentMode {
	//	if len(os.Args) >= 4 {
	//		makeDockment(os.Args[2], os.Args[3])
	//	} else if len(os.Args) >= 3 {
	//		makeDockment(os.Args[2], "")
	//	} else {
	//		makeDockment("", "")
	//	}
	//	os.Exit(0)
	//}

	running = true

	if webAuthChecker == nil {
		SetAuthChecker(DefaultAuthChecker)
	}

	if len(os.Args) > 1 {
		for i := 1; i < len(os.Args); i++ {
			if strings.ContainsRune(os.Args[i], ':') {
				_ = os.Setenv("SERVICE_LISTEN", os.Args[i])
			} else if strings.ContainsRune(os.Args[i], '=') {
				a := strings.SplitN(os.Args[i], "=", 2)
				_ = os.Setenv(a[0], a[1])
			}
			//else {
			//	_ = os.Setenv("LOG_FILE", os.Args[i])
			//}
		}
	}
	if !inited {
		Init()
		discover.Init()
	}

	listenLines := strings.Split(Config.Listen, "|")
	as.listens = make([]Listen, 0)
	for _, listenStr := range listenLines {
		if listenStr == "" {
			continue
		}
		listenArr := u.SplitWithoutNone(listenStr, ",")
		//listen := Listen{httpVersion: Config.HttpVersion, keyFile: Config.KeyFile, certFile: Config.CertFile}
		listen := Listen{addr: "", protocol: "http"}
		//keyFileOk := false
		for i, s := range listenArr {
			if i == 0 {
				listen.addr = s
				if strings.IndexRune(listen.addr, ':') == -1 {
					listen.addr = ":" + listen.addr
				}
				if listen.addr == ":" {
					// 未指定监听，默认使用h2c协议运行在随机端口
					listen.protocol = "h2c"
				} else if strings.HasSuffix(s, "443") {
					// 尾号为443的端口，默认使用https协议
					listen.protocol = "https"
				}
			} else if s == "http" || s == "https" || s == "h2" || s == "h2c" {
				listen.protocol = s
			}
			//} else {
			//	intValue, err := strconv.Atoi(s)
			//	if err == nil && (intValue == 1 && intValue <= 2) {
			//		listen.httpVersion = intValue
			//	//} else {
			//	//	if !keyFileOk {
			//	//		keyFileOk = true
			//	//		listen.keyFile = s
			//	//	} else {
			//	//		listen.certFile = s
			//	//	}
			//	}
			//}
		}
		as.listens = append(as.listens, listen)
	}

	serverProto = as.listens[0].protocol
	if serverProto == "https" || serverProto == "h2" {
		serverProtoName = "https"
	}
	//if Config.HttpVersion == 1 {
	//	if Config.CertFile == "" {
	//		serverProto = "http"
	//	} else {
	//		serverProto = "https"
	//	}
	//} else {
	//	Config.HttpVersion = 2
	//	if Config.CertFile == "" {
	//		serverProto = "h2c"
	//	} else {
	//		serverProto = "h2"
	//	}
	//}

	logInfo("starting")

	//if Config.RwTimeout > 0 {
	//	srv.ReadTimeout = time.Duration(Config.RwTimeout) * time.Millisecond
	//	srv.ReadHeaderTimeout = time.Duration(Config.RwTimeout) * time.Millisecond
	//	srv.WriteTimeout = time.Duration(Config.RwTimeout) * time.Millisecond
	//}

	for k, listen := range as.listens {
		listener, err := net.Listen("tcp", listen.addr)
		if err != nil {
			logError(err.Error())
			as.startChan <- false
			return
		}
		as.listens[k].listener = listener
	}

	as.stopChan = make(chan bool, len(as.listens))

	closeChan := make(chan os.Signal, 1)
	//signal.Notify(closeChan, os.Interrupt, syscall.SIGTERM)
	signal.Notify(closeChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		<-closeChan
		as.stop()
	}()

	addrInfo := as.listens[0].listener.Addr().(*net.TCPAddr)
	ip := addrInfo.IP
	port := addrInfo.Port
	if !ip.IsGlobalUnicast() {
		// 如果监听的不是外部IP，使用第一个外部IP
		addrs, _ := net.InterfaceAddrs()
		for _, a := range addrs {
			an := a.(*net.IPNet)
			// 显式匹配网段
			if Config.IpPrefix != "" && strings.HasPrefix(an.IP.To4().String(), Config.IpPrefix) {
				ip = an.IP.To4()
				break
			}

			// 忽略 Docker 私有网段，匹配最后一个
			if an.IP.IsGlobalUnicast() && !strings.HasPrefix(an.IP.To4().String(), "172.17.") {
				ip = an.IP.To4()
			}
		}
	}
	serverAddr = fmt.Sprintf("%s:%d", ip.String(), port)

	logInfo("starting discover")
	if discover.Start(serverAddr) == false {
		logError("failed to start discover")
		as.stop()
		return
	}

	for _, ts := range timerServers {
		logInfo("starting timer server", "name", ts.name, "interval", ts.intervalDuration/time.Second)
		if ts.start != nil {
			ts.start()
		}

		ts.running = true
		go runTimerServer(ts)
	}

	// 信息记录到 pid file
	serviceInfo.pid = os.Getpid()
	serviceInfo.protocol = serverProto
	checkAddr := serverAddr
	if Config.CheckDomain != "" {
		checkAddr = fmt.Sprintf("%s:%d", Config.CheckDomain, port)
	}
	//if as.listens[0].certFile != "" && as.listens[0].keyFile != "" {
	if strings.Contains(as.listens[0].addr, "443") {
		serviceInfo.baseUrl = "https://" + checkAddr
	} else {
		serviceInfo.baseUrl = "http://" + checkAddr
	}
	serviceInfo.save()

	Restful(0, "HEAD", "/__CHECK__", defaultChecker, "check service is available")

	logInfo("started", "cpuCoreNum", Config.Cpu, "maxMemory", Config.Memory)
	if Config.StatisticTime {
		// 统计请求个阶段的处理时间
		timeStatistic = NewTimeStatistic(serverLogger)
	}

	as.Addr = serverAddr
	as.startChan <- true
	// 11
	rh := routeHandler{}
	as.routeHandler = rh

	for k := range as.listens {
		listen := as.listens[k]
		go func() {
			srv := &http.Server{
				//Addr:    listen.addr,
				Handler: &rh,
				TLSConfig: &tls.Config{
					GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
						if certSet := Config.SSL[info.ServerName]; certSet != nil {
							cert, err := tls.LoadX509KeyPair(certSet.CertFile, certSet.KeyFile)
							if err == nil {
								return &cert, nil
							} else {
								return nil, err
							}
						} else {
							return nil, errors.New("no cert configured for " + info.ServerName)
						}
					},
				},
			}
			if Config.KeepaliveTimeout > 0 {
				srv.IdleTimeout = time.Duration(Config.KeepaliveTimeout) * time.Millisecond
			}

			if listen.protocol == "h2" || listen.protocol == "h2c" {
				//srv.TLSConfig = &tls.Config{NextProtos: []string{"http/2", "http/1.1"}}
				s2 := &http2.Server{}
				err := http2.ConfigureServer(srv, s2)
				if err != nil {
					logError(err.Error())
					return
				}

				//if listen.certFile != "" && listen.keyFile != "" {
				//if listen.certFile != "" && listen.keyFile != "" {
				if listen.protocol == "h2" {
					//err := srv.ServeTLS(listen.listener, listen.certFile, listen.keyFile)
					err := srv.ServeTLS(listen.listener, "", "")
					if err != nil {
						logError(err.Error())
					}
				} else {
					for {
						conn, err := listen.listener.Accept()
						if err != nil {
							if strings.Contains(err.Error(), "use of closed network connection") {
								break
							}
							logError(err.Error())
							continue
						}
						go s2.ServeConn(conn, &http2.ServeConnOpts{BaseConfig: srv})
					}
				}
			} else {
				var err error
				if listen.protocol == "https" {
					//err = srv.ServeTLS(listen.listener, listen.certFile, listen.keyFile)
					err = srv.ServeTLS(listen.listener, "", "")
				} else {
					err = srv.Serve(listen.listener)
				}
				if err != nil && strings.Index(err.Error(), "use of closed network connection") == -1 {
					logError(err.Error())
				}
			}
			as.stopChan <- true
		}()
	}
	return
}

func runTimerServer(ts *timerServer) {
	defer func() {
		if err := recover(); err != nil {
			logError(u.String(err))
			if ts.running {
				logError("restart timer server", "serverName", ts.name)
				runTimerServer(ts)
			}
		}
	}()

	for {
		if !ts.running {
			break
		}

		for i := 0; i < ts.intervalTimes; i++ {
			time.Sleep(time.Millisecond * 500)
			if !ts.running {
				break
			}
		}
		if !ts.running {
			break
		}

		if ts.run != nil {
			ts.run(&ts.running)
		}

		if !ts.running {
			break
		}
	}

	if ts.stopChan != nil {
		ts.stopChan <- true
	}
}

func IsRunning() bool {
	return running
}

//type ttOut struct {
//	Result
//	Data string
//}

//func tt() (out ttOut) {
//	out.Data = "hello"
//	out.OK()
//	return
//}

func (r *Result) OK(argots ...Argot) {
	r.Ok = true
	if len(argots) > 0 {
		r.Argot = argots[0]
	}
}

func (r *Result) Failed(message string, argots ...Argot) {
	r.Ok = false
	r.Message = message
	if len(argots) > 0 {
		r.Argot = argots[0]
	}
}

func (r *Result) Done(ok bool, failedMessage string, argots ...Argot) {
	r.Ok = ok
	if !ok {
		r.Message = failedMessage
		if len(argots) > 0 {
			r.Argot = argots[0]
		}
	}
}

func (r *CodeResult) OK() {
	r.Code = 1
}

func (r *CodeResult) Failed(code int, message string) {
	r.Code = code
	r.Message = message
}

func MakeArgots(argots interface{}) {
	v := reflect.ValueOf(argots)
	if v.Kind() != reflect.Ptr {
		log.DefaultLogger.Error("not point on s.MakeArgots")
		return
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		log.DefaultLogger.Error("not struct on s.MakeArgots")
		return
	}

	for i := v.NumField() - 1; i >= 0; i-- {
		f := v.Field(i)
		if f.Type().String() == "s.Argot" {
			t := v.Type().Field(i)
			if f.CanSet() {
				f.SetString(t.Name)
				_argots = append(_argots, ArgotInfo{
					Name: Argot(t.Name),
					Memo: t.Tag.Get("memo"),
				})
			}
		}
	}
}

//var redisLock = sync.Mutex{}
func Subscribe(channel string, reset func(), received func([]byte)) bool {
	if !_rdStarted {
		//redisLock.Lock()
		_rdStarted = true
		//redisLock.Unlock()
		getPubSubRedis().Start()
	}

	return getPubSubRedis().Subscribe(channel, reset, received)
}

func Publish(channel, data string) bool {
	if !_rdStarted {
		//redisLock.Lock()
		_rdStarted = true
		//redisLock.Unlock()
		getPubSubRedis().Start()
	}

	return getPubSubRedis().PUBLISH(channel, data)
}
