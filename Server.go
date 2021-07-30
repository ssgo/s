package s

import (
	"fmt"
	"github.com/ssgo/config"
	"github.com/ssgo/discover"
	"github.com/ssgo/httpclient"
	"github.com/ssgo/log"
	"github.com/ssgo/redis"
	"github.com/ssgo/standard"
	"github.com/ssgo/u"
	"golang.org/x/net/http2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Arr = []interface{}

type Map = map[string]interface{}

var inited = false
var running = false

type serviceConfig struct {
	Listen                        string
	HttpVersion                   int
	KeepaliveTimeout              int
	NoLogGets                     bool
	NoLogHeaders                  string
	NoLogInputFields              bool
	LogInputArrayNum              int
	NoLogOutputFields             string
	LogOutputArrayNum             int
	LogWebsocketAction            bool
	Compress                      bool
	CompressMinSize               int
	CompressMaxSize               int
	CertFile                      string
	KeyFile                       string
	CheckDomain                   string
	AccessTokens                  map[string]*int
	RewriteTimeout                int
	AcceptXRealIpWithoutRequestId bool
	StatisticTime                 bool
	StatisticTimeInterval         int
	Fast                          bool
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

var _argots = make([]Argot, 0)

var Config = serviceConfig{}

var accessTokens = map[string]*int{}

//var callTokens = map[string]*string{}

//type Call struct {
//	AccessToken string
//	Host        string
//	Timeout     int
//	HttpVersion int
//	WithSSL     bool
//}

var _rd *redis.Redis

var noLogHeaders = map[string]bool{}

//var encryptLogFields = map[string]bool{}
var noLogOutputFields = map[string]bool{}

var serverId = u.ShortUniqueId()
var serverStartTime = time.Now()
var serverLogger = log.New(serverId)

var serverAddr string
var serverProto string
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
	intervalTimes := int(interval / time.Millisecond * 500)
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
func DefaultAuthChecker(authLevel int, url *string, in map[string]interface{}, request *http.Request, response *Response) (pass bool, sessionObject interface{}) {
	if authLevel == 0 {
		return true, nil
	}
	setAuthLevel := accessTokens[request.Header.Get("Access-Token")]
	return setAuthLevel != nil && *setAuthLevel >= authLevel, nil
}

func defaultChecker(request *http.Request, response http.ResponseWriter) {
	if request.Header.Get("Pid") != strconv.Itoa(serviceInfo.pid) {
		response.WriteHeader(ResponseCodeHeartbeatPidError)
		return
	}

	var ok bool
	if checker != nil {
		ok = running && checker(request)
	} else {
		ok = running
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
	listener    net.Listener
	addr        string
	httpVersion int
	certFile    string
	keyFile     string
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

		if discover.IsClient() || discover.IsServer() {
			logInfo("stopping discover")
			discover.Stop()
		}
		logInfo("stopping router")
		as.routeHandler.Stop()

		for _, ts := range timerServers {
			logInfo("stopping timer server", "name", ts.name, "interval", ts.intervalDuration)
			if ts.stop != nil {
				ts.stop()
			}
			ts.running = false
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
	r := as.clientPool.Do(method, fmt.Sprintf("%s://%s%s", u.StringIf(as.listens[0].certFile != "" && as.listens[0].keyFile != "", "https", "http"), as.Addr, path), data, headers...)
	if usedSessionIdKey != "" && r.Response != nil && r.Response.Header != nil && r.Response.Header.Get(usedSessionIdKey) != "" {
		as.clientPool.SetGlobalHeader(usedSessionIdKey, r.Response.Header.Get(usedSessionIdKey))
	}
	return r
}

func (as *AsyncServer) SetGlobalHeader(k, v string) {
	as.clientPool.SetGlobalHeader(k, v)
}

//func AsyncStart() *AsyncServer {
//	return asyncStart(2)
//}
//func AsyncStart1() *AsyncServer {
//	return asyncStart(1)
//}
func AsyncStart() *AsyncServer {
	as := &AsyncServer{startChan: make(chan bool, 1)}
	as.Start()
	//start(as)
	<-as.startChan
	if Config.HttpVersion == 1 || Config.CertFile != "" {
		as.clientPool = httpclient.GetClient(time.Duration(Config.RewriteTimeout) * time.Millisecond)
	} else {
		as.clientPool = httpclient.GetClientH2C(time.Duration(Config.RewriteTimeout) * time.Millisecond)
	}
	return as
}

func Init() {
	inited = true
	config.LoadConfig("service", &Config)

	// safe AccessTokens
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
		Config.LogInputArrayNum = 100
	}

	if Config.LogOutputArrayNum <= 0 {
		Config.LogOutputArrayNum = 2
	}

	if Config.HttpVersion == 1 {
		if Config.CertFile == "" {
			serverProto = "http"
		} else {
			serverProto = "https"
		}
	} else {
		Config.HttpVersion = 2
		if Config.CertFile == "" {
			serverProto = "h2c"
		} else {
			serverProto = "h2"
		}
	}

	if Config.Listen == "" {
		Config.Listen = ":"
	}

	serverAddr = Config.Listen
}

var timeStatistic *TimeStatistician

func Start() {
	//start(nil)
	AsyncStart().Wait()
}

func (as *AsyncServer) Start() {
	log.Start()
	logInfo("logger started")

	// document must after registers
	if inDocumentMode {
		if len(os.Args) >= 4 {
			makeDockment(os.Args[2], os.Args[3])
		} else if len(os.Args) >= 3 {
			makeDockment(os.Args[2], "")
		} else {
			makeDockment("", "")
		}
		os.Exit(0)
	}

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
		listenArr := strings.Split(listenStr, ",")
		listen := Listen{httpVersion: Config.HttpVersion, keyFile: Config.KeyFile, certFile: Config.CertFile}
		keyFileOk := false
		for i, s := range listenArr {
			if i == 0 {
				listen.addr = s
				if strings.IndexRune(listen.addr, ':') == -1 {
					listen.addr = ":" + listen.addr
				}
			} else {
				intValue, err := strconv.Atoi(s)
				if err == nil && (intValue == 1 && intValue <= 2) {
					listen.httpVersion = intValue
				} else {
					if !keyFileOk {
						keyFileOk = true
						listen.keyFile = s
					} else {
						listen.certFile = s
					}
				}
			}
		}
		as.listens = append(as.listens, listen)
	}

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
			// 忽略 Docker 私有网段
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
		logInfo("starting timer server", "name", ts.name, "interval", ts.intervalDuration)
		if ts.start != nil {
			ts.start()
		}

		ts.running = true
		go runTimerServer(ts)
	}

	// 信息记录到 pid file
	serviceInfo.pid = os.Getpid()
	serviceInfo.httpVersion = Config.HttpVersion
	checkAddr := serverAddr
	if Config.CheckDomain != "" {
		checkAddr = fmt.Sprintf("%s:%d", Config.CheckDomain, port)
	}
	if as.listens[0].certFile != "" && as.listens[0].keyFile != "" {
		serviceInfo.baseUrl = "https://" + checkAddr
	} else {
		serviceInfo.baseUrl = "http://" + checkAddr
	}
	serviceInfo.save()

	Restful(0, "HEAD", "/__CHECK__", defaultChecker)

	logInfo("started")
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
			}
			if Config.KeepaliveTimeout > 0 {
				srv.IdleTimeout = time.Duration(Config.KeepaliveTimeout) * time.Millisecond
			}

			if listen.httpVersion == 2 {
				//srv.TLSConfig = &tls.Config{NextProtos: []string{"http/2", "http/1.1"}}
				s2 := &http2.Server{}
				err := http2.ConfigureServer(srv, s2)
				if err != nil {
					logError(err.Error())
					return
				}

				if listen.certFile != "" && listen.keyFile != "" {
					err := srv.ServeTLS(listen.listener, listen.certFile, listen.keyFile)
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
				if listen.certFile != "" && listen.keyFile != "" {
					err = srv.ServeTLS(listen.listener, listen.certFile, listen.keyFile)
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

		if ts.run != nil {
			ts.run(&ts.running)
		}

		if !ts.running {
			break
		}
		for i:=0; i<ts.intervalTimes; i++ {
			time.Sleep(time.Millisecond * 500)
			if !ts.running {
				break
			}
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

func (r *CodeResult) OK() {
	r.Code = 1
}

func (r *CodeResult) Failed2(code int, message string) {
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
				_argots = append(_argots, Argot(t.Name))
			}
		}
	}
}
