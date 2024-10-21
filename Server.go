package s

import (
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

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
)

type Arr = []any

type Map = map[string]any

// var name = "Noname Server"
var version = "unset version"
var inited = false
var running = false
var workPath string // 工作路径，默认为可执行文件的当前目录

func SetWorkPath(p string) {
	workPath = p
}

type ServiceConfig struct {
	Listen string              // 监听端口（|隔开多个监听）（,隔开多个选项）（如果不指定IP则监听在0.0.0.0，如果不指定端口则使用h2c协议监听在随机端口，80端口默认使用http协议，443端口默认使用https协议），例如 80,http|443|443:h2|127.0.0.1:8080,h2c
	SSL    map[string]*CertSet // SSL证书配置，key为域名，value为cert和key的文件路径
	//KeepaliveTimeout              int                 // 连接允许空闲的最大时间，单位ms，默认值：15000
	NoLogGets                     bool            // 不记录GET请求的日志
	NoLogHeaders                  string          // 不记录请求头中包含的这些字段，多个字段用逗号分隔，默认不记录：Accept,Accept-Encoding,Cache-Control,Pragma,Connection,Upgrade-Insecure-Requests
	LogInputArrayNum              int             // 请求字段中容器类型（数组、Map）在日志打印个数限制 默认为10个，多余的数据将不再日志中记录
	LogInputFieldSize             int             // 请求字段中单个字段在日志打印长度限制 默认为500个字符，多余的数据将不再日志中记录
	NoLogOutputFields             string          // 不记录响应字段中包含的这些字段（key名），多个字段用逗号分隔
	LogOutputArrayNum             int             // 响应字段中容器类型（数组、Map）在日志打印个数限制 默认为3个，多余的数据将不再日志中记录
	LogOutputFieldSize            int             // 响应字段中单个字段在日志打印长度限制 默认为100个字符，多余的数据将不再日志中记录
	LogWebsocketAction            bool            // 记录Websocket中每个Action的请求日志，默认不记录
	Compress                      bool            // 是否启用压缩，默认不启用
	CompressMinSize               int             // 小于设定值的应答内容将不进行压缩，默认值：1024
	CompressMaxSize               int             // 大于设定值的应答内容将不进行压缩，默认值：4096000
	CheckDomain                   string          // 心跳检测时使用域名，，默认使用IP地址，心跳检测使用 HEAD /__CHECK__ 请求，应答 299 表示正常，593 表示异常
	AccessTokens                  map[string]*int // 请求接口时使用指定的Access-Token进行验证，值为Token对应的auth-level
	RedirectTimeout               int             // proxy和discover发起请求时的超时时间，单位ms，默认值：10000
	AcceptXRealIpWithoutRequestId bool            // 是否允许头部没有携带请求ID的X-Real-IP信息，默认不允许（防止伪造客户端IP）
	StatisticTime                 bool            // 是否开启请求时间统计，默认不开启
	StatisticTimeInterval         int             // 统计时间间隔，单位ms，默认值：10000
	Fast                          bool            // 是否启用快速模式（为了追求性能牺牲一部分特性），默认不启用
	MaxUploadSize                 int64           // 最大上传文件大小（multipart/form-data请求的总空间），单位字节，默认值：104857600
	IpPrefix                      string          // discover服务发现时指定使用的IP网段，默认排除 172.17.（Docker）
	Cpu                           int             // CPU占用的核数，默认为0，即不做限制
	Memory                        int             // 内存（单位M），默认为0，即不做限制
	CpuMonitor                    bool            // 在日志中记录CPU使用情况，默认不开启
	MemoryMonitor                 bool            // 在日志中记录内存使用情况，默认不开启
	CpuLimitValue                 uint            // CPU超过最高占用值（10-100）超过次数将自动重启（如果CpuMonitor开启的话），默认100
	MemoryLimitValue              uint            // 内存超过最高占用值（10-100）超过次数将自动重启（如果MemoryMonitor开启的话），默认95
	CpuLimitTimes                 uint            // CPU超过最高占用值超过次数（1-100）将报警（如果CpuMonitor开启的话），默认6（即30秒内连续6次）
	MemoryLimitTimes              uint            // 内存超过最高占用值超过次数（1-100）将报警（如果MemoryMonitor开启的话），默认6（即30秒内连续6次）
	CookieScope                   string          // 启用Session时Cookie的有效范围，host|domain|topDomain，默认值为host
	IdServer                      string          // 用s.UniqueId、s.Id来生成唯一ID（雪花算法）时所需的redis服务器连接，如果不指定将不能实现跨服务的全局唯一
	KeepKeyCase                   bool            // 是否保持Key的首字母大小写？默认一律使用小写
	IndexFiles                    []string        // 访问静态文件时的索引文件，默认为 index.html
	IndexDir                      bool            // 访问目录时显示文件列表
	ReadTimeout                   int             // 读取请求的超时时间，单位ms
	ReadHeaderTimeout             int             // 读取请求头的超时时间，单位ms
	WriteTimeout                  int             // 响应写入的超时时间，单位ms
	IdleTimeout                   int             // 连接空闲超时时间，单位ms
	MaxHeaderBytes                int             // 请求头的最大字节数
	MaxHandlers                   int             // 每个连接的最大处理程序数量
	MaxConcurrentStreams          uint32          // 每个连接的最大并发流数量
	MaxDecoderHeaderTableSize     uint32          // 解码器头表的最大大小
	MaxEncoderHeaderTableSize     uint32          // 编码器头表的最大大小
	MaxReadFrameSize              uint32          // 单个帧的最大读取大小
	MaxUploadBufferPerConnection  int32           // 每个连接的最大上传缓冲区大小
	MaxUploadBufferPerStream      int32           // 每个流的最大上传缓冲区大小
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

var Config = ServiceConfig{}

var accessTokens = map[string]*int{}
var accessTokensLock = sync.RWMutex{}

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

//func SetName(serverName string) {
//	name = serverName
//}

func SetVersion(serverVersion string) {
	version = serverVersion
}

func getRedis1() *redis.Redis {
	if _rd == nil && Config.IdServer != "" {
		_rd = redis.GetRedis(Config.IdServer, ServerLogger)
	}
	return getRedis2()
}

func getRedis2() *redis.Redis {
	if _rd == nil && discover.Config.Registry != "" && discover.Config.Registry != standard.DiscoverDefaultRegistry {
		_rd = redis.GetRedis(discover.Config.Registry, ServerLogger)
	}
	return _rd
}

func getPubSubRedis2() *redis.Redis {
	if _rd2 == nil {
		rd := getRedis2()
		if rd == nil {
			return nil
		}
		confForPubSub := *rd.Config
		confForPubSub.IdleTimeout = -1
		confForPubSub.ReadTimeout = -1
		_rd2 = redis.NewRedis(&confForPubSub, ServerLogger)
	}
	return _rd2
}

var noLogHeaders = map[string]bool{}

// var encryptLogFields = map[string]bool{}
var noLogOutputFields = map[string]bool{}

var serverId = u.UniqueId()
var serverStartTime = time.Now()
var ServerLogger = log.New(serverId)

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
	if interval < time.Millisecond*100 {
		interval = time.Millisecond * 100
	}
	intervalTimes := int(interval / (time.Millisecond * 100))
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

func logInfo(info string, extra ...any) {
	ServerLogger.Server(info, discover.Config.App, discover.Config.Weight, serverAddr, serverProto, serverStartTime, extra...)
}

func logError(error string, extra ...any) {
	ServerLogger.ServerError(error, discover.Config.App, discover.Config.Weight, serverAddr, serverProto, serverStartTime, extra...)
}

func SetChecker(ck func(request *http.Request) bool) {
	checker = ck
}

func GetServerAddr() string {
	return serverAddr
}

func SetAuthTokenLevel(authToken string, authLevel int) {
	accessTokensLock.Lock()
	if accessTokens == nil {
		accessTokens = map[string]*int{}
	}
	accessTokens[authToken] = &authLevel
	accessTokensLock.Unlock()
}

func GetAuthTokenLevel(authToken string) int {
	accessTokensLock.RLock()
	authLevel := accessTokens[authToken]
	accessTokensLock.RUnlock()
	if authLevel != nil {
		return *authLevel
	}
	return 0
}

func GetLocalPublicIP() []string {
	ips := make([]string, 0)
	ipAddresses, _ := net.InterfaceAddrs()
	for _, a := range ipAddresses {
		an := a.(*net.IPNet)
		if Config.IpPrefix != "" && strings.HasPrefix(an.IP.To16().String(), Config.IpPrefix) {
			// 根据配置匹配指定网段
			ips = append(ips, an.IP.To16().String())
		} else if Config.IpPrefix != "" && strings.HasPrefix(an.IP.To4().String(), Config.IpPrefix) {
			// 根据配置匹配指定网段
			ips = append(ips, an.IP.To4().String())
		} else if an.IP.IsGlobalUnicast() && !strings.HasPrefix(an.IP.To4().String(), "172.17.") {
			// 忽略 Docker 私有网段，进行自动匹配
			ips = append(ips, an.IP.To4().String())
		}
	}
	return ips
}

// noinspection GoUnusedParameter
func DefaultAuthChecker(authLevel int, logger *log.Logger, url *string, in map[string]any, request *Request, response *Response, options *WebServiceOptions) (pass bool, sessionObject any) {
	if authLevel == 0 {
		return true, nil
	}
	setAuthLevel := GetAuthTokenLevel(request.Header.Get("Access-Token"))
	return setAuthLevel >= authLevel, nil
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
	if ok && Config.CpuMonitor && cpuOverTimes >= Config.CpuLimitTimes {
		// CPU 1分钟连续6次达到 100%
		ok = false
	}
	if ok && Config.MemoryMonitor && memoryOutTimes >= Config.MemoryLimitTimes {
		// 内存 1分钟连续6次达到 95%
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
	closeChan    chan os.Signal
	listens      []Listen
	Addr         string
	Proto        string
	ProtoName    string
	clientPool   *httpclient.ClientPool
	routeHandler routeHandler
	waited       bool
	onStop       func()
	onStopped    func()
}

func (as *AsyncServer) OnStop(f func()) {
	as.onStop = f
}

func (as *AsyncServer) OnStopped(f func()) {
	as.onStopped = f
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
			logInfo("stopping timer server", "name", ts.name, "interval", ts.intervalDuration/time.Millisecond)
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
			logInfo("waiting timer server", "name", ts.name, "interval", ts.intervalDuration/time.Millisecond)
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

		// 清楚内存数据
		resetWebServiceMemory()
		resetWebSocketMemory()
		resetProxyMemory()
		resetRewriteMemory()
		resetStarterMemory()
		resetStaticMemory()
		resetUtilityMemory()
		resetServerMemory()

		serviceInfo.remove()
		logInfo("stopped")

		// 最后关闭日志服务
		logInfo("logger stopped")
		// ServerLogger = nil
		log.Stop()
		log.Wait()
	}
}

func resetServerMemory() {
	version = "unset version"
	inited = false
	running = false
	workPath = ""
	_argots = make([]ArgotInfo, 0)
	Config = ServiceConfig{}
	accessTokens = map[string]*int{}
	_cpuOverTimes = 0
	_memoryOutTimes = 0
	_rd = nil
	_rd2 = nil
	_rdStarted = false
	noLogHeaders = map[string]bool{}
	noLogOutputFields = map[string]bool{}
	serverId = UniqueId()
	serverStartTime = time.Now()
	serverAddr = ""
	serverProto = "http"
	serverProtoName = "http"
	checker = nil
	shutdownHooks = make([]func(), 0)
	timerServers = make([]*timerServer, 0)
}

func (as *AsyncServer) stop() {
	for _, listen := range as.listens {
		_ = listen.listener.Close()
	}
	if as.onStop != nil {
		as.onStop()
	}
	running = false
}

func (as *AsyncServer) Stop() {
	//as.stop()
	if as.closeChan != nil {
		signal.Stop(as.closeChan)
		as.closeChan <- syscall.SIGTERM
	} else {
		as.stop()
	}
	as.Wait()
	if as.onStopped != nil {
		as.onStopped()
	}
}

func (as *AsyncServer) Get(path string, headers ...string) *httpclient.Result {
	return as.Do("GET", path, nil, headers...)
}
func (as *AsyncServer) Post(path string, data any, headers ...string) *httpclient.Result {
	return as.Do("POST", path, data, headers...)
}
func (as *AsyncServer) Put(path string, data any, headers ...string) *httpclient.Result {
	return as.Do("PUT", path, data, headers...)
}
func (as *AsyncServer) Delete(path string, data any, headers ...string) *httpclient.Result {
	return as.Do("DELETE", path, data, headers...)
}
func (as *AsyncServer) Head(path string, headers ...string) *httpclient.Result {
	return as.Do("HEAD", path, nil, headers...)
}

func (as *AsyncServer) Do(method, path string, data any, headers ...string) *httpclient.Result {
	//r := as.clientPool.Do(method, fmt.Sprintf("%s://%s%s", u.StringIf(as.listens[0].certFile != "" && as.listens[0].keyFile != "", "https", "http"), as.Addr, path), data, headers...)
	//fmt.Println("= ========", serverProtoName, "[["+as.Addr+"]]")
	r := as.clientPool.Do(method, fmt.Sprintf("%s://%s%s", serverProtoName, as.Addr, path), data, headers...)
	//if usedSessionIdKey != "" && r.Response != nil && r.Response.Header != nil && r.Response.Header.Get(usedSessionIdKey) != "" {
	//	as.clientPool.SetGlobalHeader(usedSessionIdKey, r.Response.Header.Get(usedSessionIdKey))
	//}
	return r
}
func (as *AsyncServer) ManualDo(method, path string, data any, headers ...string) *httpclient.Result {
	r := as.clientPool.ManualDo(method, fmt.Sprintf("%s://%s%s", serverProtoName, as.Addr, path), data, headers...)
	return r
}

func (as *AsyncServer) SetGlobalHeader(k, v string) {
	as.clientPool.SetGlobalHeader(k, v)
}

func (as *AsyncServer) NewClient(timeout time.Duration) *Client {
	c := &Client{addr: as.Addr}
	if as.listens[0].protocol != "h2c" {
		c.clientPool = httpclient.GetClient(timeout)
	} else {
		c.clientPool = httpclient.GetClientH2C(timeout)
	}
	return c
}

type Client struct {
	addr       string
	clientPool *httpclient.ClientPool
}

func (c *Client) Get(path string, headers ...string) *httpclient.Result {
	return c.Do("GET", path, nil, headers...)
}

func (c *Client) Post(path string, data any, headers ...string) *httpclient.Result {
	return c.Do("POST", path, data, headers...)
}

func (c *Client) Put(path string, data any, headers ...string) *httpclient.Result {
	return c.Do("PUT", path, data, headers...)
}

func (c *Client) Delete(path string, data any, headers ...string) *httpclient.Result {
	return c.Do("DELETE", path, data, headers...)
}

func (c *Client) Head(path string, headers ...string) *httpclient.Result {
	return c.Do("HEAD", path, nil, headers...)
}

func (c *Client) Do(method, path string, data any, headers ...string) *httpclient.Result {
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
		as.clientPool = httpclient.GetClient(time.Duration(Config.RedirectTimeout) * time.Millisecond)
	} else {
		as.clientPool = httpclient.GetClientH2C(time.Duration(Config.RedirectTimeout) * time.Millisecond)
	}
	return as
}

func Init() {
	inited = true
	initStarter()

	config.LoadConfig("service", &Config)

	for k, v := range Config.AccessTokens {
		SetAuthTokenLevel(k, *v)
	}
	Config.AccessTokens = nil

	//if Config.KeepaliveTimeout <= 0 {
	//	Config.KeepaliveTimeout = 15000
	//}

	if Config.CompressMinSize <= 0 {
		Config.CompressMinSize = 1024
	}

	if Config.CompressMaxSize <= 0 {
		Config.CompressMaxSize = 4096000
	}

	if Config.RedirectTimeout <= 0 {
		Config.RedirectTimeout = 10000
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
		Config.MaxUploadSize = 1024 * 1024 * 100
	}

	if Config.Listen == "" {
		Config.Listen = ":"
	}

	serverAddr = Config.Listen

	if Config.CookieScope == "" {
		Config.CookieScope = "host"
	}

	if Config.IndexFiles == nil {
		Config.IndexFiles = []string{"index.html"}
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

	if Config.CpuLimitValue <= 0 || Config.CpuLimitValue > 100 {
		Config.CpuLimitValue = 100
	}
	if Config.MemoryLimitValue <= 0 || Config.MemoryLimitValue > 100 {
		Config.MemoryLimitValue = 95
	}
	if Config.CpuLimitTimes <= 0 {
		Config.CpuLimitTimes = 6
	}
	if Config.MemoryLimitTimes <= 0 {
		Config.MemoryLimitTimes = 6
	}
	//ms := runtime.MemStats{}
	//runtime.ReadMemStats(&ms)
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
	ServerLogger = log.New(serverId)
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

	// 监控
	if Config.CpuMonitor || Config.MemoryMonitor {
		var serviceProcess *process.Process
		var cpuCounter *Counter
		var memoryCounter *Counter
		var memoryStat = runtime.MemStats{}
		NewTimerServer("serverMonitor", time.Second*5, func(serverRunning *bool) {
			if Config.MemoryMonitor {
				runtime.ReadMemStats(&memoryStat)
				memoryUsed := byteToM(memoryStat.Sys)
				memoryPercent := math.Round(float64(memoryUsed)/float64(Config.Memory)*10000) / 100
				if memoryPercent >= 60 {
					memoryCounter.AddFailed(memoryPercent)
				} else {
					memoryCounter.Add(memoryPercent)
				}

				if memoryPercent >= float64(Config.MemoryLimitValue) {
					_memoryOutTimes++
				} else {
					_memoryOutTimes = 0
				}

				if memoryCounter.Times >= 12 {
					memoryCounter.Count()
					memoryInfo := []any{"memoryUsed", memoryUsed, "limit", Config.Memory, "memoryPercent", memoryPercent, "heapInuse", byteToM(memoryStat.HeapInuse), "heapIdle", byteToM(memoryStat.HeapIdle), "stackInuse", byteToM(memoryStat.StackInuse), "heapInuse", byteToM(memoryStat.HeapInuse), "pauseTotalNs", memoryStat.PauseTotalNs, "numGC", memoryStat.NumGC, "numForcedGC", memoryStat.NumForcedGC}
					if memoryPercent >= 100 {
						logError("out of memory", memoryInfo...)
					} else if memoryPercent >= 80 {
						logError("memory danger", memoryInfo...)
					} else if memoryPercent >= 60 {
						logError("memory warning", memoryInfo...)
					}
					ServerLogger.Statistic(serverId, discover.Config.App, "memoryCount", memoryCounter.StartTime, memoryCounter.EndTime, memoryCounter.Times, memoryCounter.Failed, memoryCounter.Avg, memoryCounter.Min, memoryCounter.Max, memoryInfo...)
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

					if cpuPercent >= float64(Config.CpuLimitValue) {
						_cpuOverTimes++
					} else {
						_cpuOverTimes = 0
					}

					if cpuCounter.Times >= 12 {
						cpuCounter.Count()
						numThreads, _ := serviceProcess.NumThreads()
						cpuInfo := []any{"threads", numThreads, "goroutine", runtime.NumGoroutine(), "limit", Config.Cpu}
						if cpuPercent >= 100 {
							logError("over load of cpu", cpuInfo...)
						} else if cpuPercent >= 80 {
							logError("cpu danger", cpuInfo...)
						} else if cpuPercent >= 60 {
							logError("cpu warning", cpuInfo...)
						}
						ServerLogger.Statistic(serverId, discover.Config.App, "cpuCount", cpuCounter.StartTime, cpuCounter.EndTime, cpuCounter.Times, cpuCounter.Failed, cpuCounter.Avg, cpuCounter.Min, cpuCounter.Max, cpuInfo...)
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
					memoryInfo := []any{"memoryUsed", memoryUsed, "limit", Config.Memory, "memoryPercent", memoryPercent, "heapInuse", byteToM(memoryStat.HeapInuse), "heapIdle", byteToM(memoryStat.HeapIdle), "stackInuse", byteToM(memoryStat.StackInuse), "heapInuse", byteToM(memoryStat.HeapInuse), "pauseTotalNs", memoryStat.PauseTotalNs, "numGC", memoryStat.NumGC, "numForcedGC", memoryStat.NumForcedGC}
					ServerLogger.Statistic(serverId, discover.Config.App, "memoryCount", memoryCounter.StartTime, memoryCounter.EndTime, memoryCounter.Times, memoryCounter.Failed, memoryCounter.Avg, memoryCounter.Min, memoryCounter.Max, memoryInfo...)
					memoryCounter.Reset()
				}
			}

			if Config.CpuMonitor && serviceProcess != nil {
				if cpuCounter.Times >= 1 {
					numThreads, _ := serviceProcess.NumThreads()
					cpuInfo := []any{"threads", numThreads, "goroutine", runtime.NumGoroutine(), "limit", Config.Cpu}
					ServerLogger.Statistic(serverId, discover.Config.App, "cpuCount", cpuCounter.StartTime, cpuCounter.EndTime, cpuCounter.Times, cpuCounter.Failed, cpuCounter.Avg, cpuCounter.Min, cpuCounter.Max, cpuInfo...)
					cpuCounter.Reset()
				}
			}
		})
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

	if len(websocketServicesList) > 0 && as.listens[0].protocol == "h2c" {
		// force http for websocket
		as.listens[0].protocol = "http"
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

	as.closeChan = make(chan os.Signal, 1)
	//signal.Notify(closeChan, os.Interrupt, syscall.SIGTERM)
	signal.Notify(as.closeChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		<-as.closeChan
		as.stop()
	}()

	addrInfo := as.listens[0].listener.Addr().(*net.TCPAddr)
	ip := addrInfo.IP
	port := addrInfo.Port
	ipStr := ip.String()
	if !ip.IsGlobalUnicast() {
		// 如果监听的不是外部IP，使用第一个外部IP
		ips := GetLocalPublicIP()
		if len(ips) > 0 {
			ipStr = ips[0]
		}
		//addrs, _ := net.InterfaceAddrs()
		//for _, a := range addrs {
		//	an := a.(*net.IPNet)
		//	// 显式匹配网段
		//	if Config.IpPrefix != "" && strings.HasPrefix(an.IP.To4().String(), Config.IpPrefix) {
		//		ip = an.IP.To4()
		//		break
		//	}
		//
		//	// 忽略 Docker 私有网段，匹配最后一个
		//	if an.IP.IsGlobalUnicast() && !strings.HasPrefix(an.IP.To4().String(), "172.17.") {
		//		ip = an.IP.To4()
		//	}
		//}
	}
	if ipStr == "" || ipStr == "<nil>" {
		ipStr = "127.0.0.1"
	}
	serverAddr = fmt.Sprintf("%s:%d", ipStr, port)

	logInfo("starting discover")
	if discover.Start(serverAddr) == false {
		logError("failed to start discover")
		as.stop()
		return
	}
	logInfo("started discover")

	for _, ts := range timerServers {
		logInfo("starting timer server", "name", ts.name, "interval", ts.intervalDuration/time.Millisecond)
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

	logInfo("started", "cpuCoreNum", Config.Cpu, "maxMemory", Config.Memory, "pid", serviceInfo.pid, "pidFile", serviceInfo.pidFile)
	if Config.StatisticTime {
		// 统计请求个阶段的处理时间
		timeStatistic = NewTimeStatistic(ServerLogger)
	}

	as.Addr = serverAddr
	as.Proto = serverProto
	as.ProtoName = serverProtoName
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
				ReadTimeout:       time.Duration(Config.ReadTimeout) * time.Millisecond,
				ReadHeaderTimeout: time.Duration(Config.ReadHeaderTimeout) * time.Millisecond,
				WriteTimeout:      time.Duration(Config.WriteTimeout) * time.Millisecond,
				IdleTimeout:       time.Duration(Config.IdleTimeout) * time.Millisecond,
				MaxHeaderBytes:    Config.MaxHeaderBytes,
			}

			if listen.protocol == "h2" || listen.protocol == "h2c" {
				//srv.TLSConfig = &tls.Config{NextProtos: []string{"http/2", "http/1.1"}}
				s2 := &http2.Server{
					MaxHandlers:                  Config.MaxHandlers,
					MaxConcurrentStreams:         Config.MaxConcurrentStreams,
					MaxDecoderHeaderTableSize:    Config.MaxDecoderHeaderTableSize,
					MaxEncoderHeaderTableSize:    Config.MaxEncoderHeaderTableSize,
					MaxReadFrameSize:             Config.MaxReadFrameSize,
					IdleTimeout:                  time.Duration(Config.IdleTimeout) * time.Millisecond,
					MaxUploadBufferPerConnection: Config.MaxUploadBufferPerConnection,
					MaxUploadBufferPerStream:     Config.MaxUploadBufferPerStream,
				}
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
			time.Sleep(time.Millisecond * 100)
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

func SleepWhileRunning(duration time.Duration) {
	intervalTimes := int(math.Ceil(float64(duration / time.Millisecond * 100)))
	for i := 0; i < intervalTimes; i++ {
		time.Sleep(time.Millisecond * 100)
		if !running {
			break
		}
	}
}

func RunTaskWhileRunning(duration time.Duration, taskFn func() bool) {
	go func() {
		SleepWhileRunning(duration)
		for IsRunning() {
			if taskFn() {
				break
			}
			SleepWhileRunning(duration)
		}
	}()
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

func MakeArgots(argots any) {
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

// var redisLock = sync.Mutex{}
func Subscribe(channel string, reset func(), received func([]byte)) bool {
	if !_rdStarted {
		//redisLock.Lock()
		_rdStarted = true
		//redisLock.Unlock()
		if rd2 := getPubSubRedis2(); rd2 != nil {
			rd2.Start()
		}
	}

	if rd2 := getPubSubRedis2(); rd2 != nil {
		return rd2.Subscribe(channel, reset, received)
	} else {
		return false
	}
}

func Publish(channel, data string) bool {
	if !_rdStarted {
		//redisLock.Lock()
		_rdStarted = true
		//redisLock.Unlock()
		if rd2 := getPubSubRedis2(); rd2 != nil {
			rd2.Start()
		}
	}

	if rd2 := getPubSubRedis2(); rd2 != nil {
		return rd2.PUBLISH(channel, data)
	} else {
		return false
	}
}
