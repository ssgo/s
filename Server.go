package s

import (
	"fmt"
	"github.com/ssgo/standard"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ssgo/config"
	"github.com/ssgo/discover"
	"github.com/ssgo/httpclient"
	"github.com/ssgo/log"
	"github.com/ssgo/u"
	"golang.org/x/net/http2"
)

type Arr = []interface{}

type Map = map[string]interface{}

var inited = false
var running = false

type serviceConfig struct {
	Listen                        string
	HttpVersion                   int
	RwTimeout                     int
	KeepaliveTimeout              int
	NoLogGets                     bool
	NoLogHeaders                  string
	NoLogInputFields              bool
	LogInputArrayNum              int
	LogOutputFields               string
	LogOutputArrayNum             int
	LogWebsocketAction            bool
	Compress                      bool
	CompressMinSize               int
	CompressMaxSize               int
	CertFile                      string
	KeyFile                       string
	AccessTokens                  map[string]*int
	CallTokens                    map[string]*string
	CallTimeout                   int
	AcceptXRealIpWithoutRequestId bool
}

var Config = serviceConfig{}

var accessTokens = map[string]*int{}
var callTokens = map[string]*string{}

//type Call struct {
//	AccessToken string
//	Host        string
//	Timeout     int
//	HttpVersion int
//	WithSSL     bool
//}

var noLogHeaders = map[string]bool{}
var encryptLogFields = map[string]bool{}
var logOutputFields = map[string]bool{}

var serverId = u.ShortUniqueId()
var serverStartTime = log.MakeLogTime(time.Now())
var serverLogger = log.New(serverId)

var serverAddr string
var serverProto string
var checker func(request *http.Request) bool

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

type AsyncServer struct {
	startChan  chan bool
	stopChan   chan bool
	listener   net.Listener
	Addr       string
	clientPool *httpclient.ClientPool
}

func (as *AsyncServer) Stop() {
	if as.listener != nil {
		_ = as.listener.Close()
	}
	if as.stopChan != nil {
		<-as.stopChan
	}
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
	r := as.clientPool.Do(method, fmt.Sprintf("%s://%s%s", u.StringIf(Config.CertFile != "" && Config.KeyFile != "", "https", "http"), as.Addr, path), data, headers...)
	if sessionKey != "" && r.Response != nil && r.Response.Header != nil && r.Response.Header.Get(sessionKey) != "" {
		as.clientPool.SetGlobalHeader(sessionKey, r.Response.Header.Get(sessionKey))
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
	as := &AsyncServer{startChan: make(chan bool), stopChan: make(chan bool)}
	go start(as)
	<-as.startChan
	if Config.HttpVersion == 1 || Config.CertFile != "" {
		as.clientPool = httpclient.GetClient(time.Duration(Config.CallTimeout) * time.Millisecond)
	} else {
		as.clientPool = httpclient.GetClientH2C(time.Duration(Config.CallTimeout) * time.Millisecond)
	}
	return as
}

func Init() {
	inited = true
	config.LoadConfig("service", &Config)

	// safe AccessTokens
	accessTokens = Config.AccessTokens
	Config.AccessTokens = nil
	callTokens = Config.CallTokens
	Config.CallTokens = nil

	if Config.KeepaliveTimeout <= 0 {
		Config.KeepaliveTimeout = 15000
	}

	if Config.CompressMinSize <= 0 {
		Config.CompressMinSize = 1024
	}

	if Config.CompressMaxSize <= 0 {
		Config.CompressMaxSize = 4096000
	}

	if Config.CallTimeout <= 0 {
		Config.CallTimeout = 10000
	}

	if Config.NoLogHeaders == "" {
		Config.NoLogHeaders = fmt.Sprint("Accept,Accept-Encoding,Accept-Language,Cache-Control,Pragma,Connection,Upgrade-Insecure-Requests")
	}
	for _, k := range strings.Split(strings.ToLower(Config.NoLogHeaders), ",") {
		noLogHeaders[strings.TrimSpace(k)] = true
		noLogHeaders[standard.DiscoverHeaderClientIp] = true
		noLogHeaders[standard.DiscoverHeaderForwardedFor] = true
		noLogHeaders[standard.DiscoverHeaderClientId] = true
		noLogHeaders[standard.DiscoverHeaderSessionId] = true
		noLogHeaders[standard.DiscoverHeaderRequestId] = true
		noLogHeaders[standard.DiscoverHeaderHost] = true
		noLogHeaders[standard.DiscoverHeaderScheme] = true
		noLogHeaders[standard.DiscoverHeaderFromApp] = true
		noLogHeaders[standard.DiscoverHeaderFromNode] = true
	}

	if Config.LogOutputFields == "" {
		Config.LogOutputFields = "code,message"
	}
	for _, k := range strings.Split(strings.ToLower(Config.LogOutputFields), ",") {
		logOutputFields[strings.TrimSpace(k)] = true
	}

	if Config.LogInputArrayNum <= 0 {
		Config.LogInputArrayNum = 0
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

	serverAddr = Config.Listen
}

func Start() {
	start(nil)
}

func start(as *AsyncServer) {
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

	if callTokens != nil && len(callTokens) > 0 {
		if discover.Config.Calls == nil {
			discover.Config.Calls = make(map[string]*discover.CallInfo)
		}
		for k, v := range callTokens {
			if discover.Config.Calls[k] == nil {
				discover.Config.Calls[k] = new(discover.CallInfo)
			}
			if discover.Config.Calls[k].Headers == nil {
				discover.Config.Calls[k].Headers = map[string]*string{}
			}
			discover.Config.Calls[k].Headers["Access-Token"] = v
		}
	}

	logInfo("starting")

	rh := routeHandler{}
	srv := &http.Server{
		Addr:    Config.Listen,
		Handler: &rh,
	}

	if Config.RwTimeout > 0 {
		srv.ReadTimeout = time.Duration(Config.RwTimeout) * time.Millisecond
		srv.ReadHeaderTimeout = time.Duration(Config.RwTimeout) * time.Millisecond
		srv.WriteTimeout = time.Duration(Config.RwTimeout) * time.Millisecond
	}

	if Config.KeepaliveTimeout > 0 {
		srv.IdleTimeout = time.Duration(Config.KeepaliveTimeout) * time.Millisecond
	}

	listener, err := net.Listen("tcp", Config.Listen)
	if as != nil {
		as.listener = listener
	}
	if err != nil {
		logError(err.Error())
		if as != nil {
			as.startChan <- false
		}
		return
	}

	closeChan := make(chan os.Signal, 2)
	signal.Notify(closeChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-closeChan
		_ = listener.Close()
	}()

	addrInfo := listener.Addr().(*net.TCPAddr)
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

	if discover.Start(serverAddr) == false {
		logError("failed to start discover")
		_ = listener.Close()
		return
	}

	// 信息记录到 pid file
	serviceInfo.pid = os.Getpid()
	serviceInfo.httpVersion = Config.HttpVersion
	if Config.CertFile != "" && Config.KeyFile != "" {
		serviceInfo.baseUrl = "https://" + serverAddr
	} else {
		serviceInfo.baseUrl = "http://" + serverAddr
	}
	serviceInfo.save()

	Restful(0, "HEAD", "/__CHECK__", defaultChecker)

	logInfo("started")
	//log.Printf("SERVER	%s	Started", serverAddr)

	if as != nil {
		as.Addr = serverAddr
		as.startChan <- true
	}
	if Config.HttpVersion == 2 {
		//srv.TLSConfig = &tls.Config{NextProtos: []string{"http/2", "http/1.1"}}
		s2 := &http2.Server{}
		err := http2.ConfigureServer(srv, s2)
		if err != nil {
			logError(err.Error())
			return
		}

		if Config.CertFile != "" && Config.KeyFile != "" {
			err := srv.ServeTLS(listener, Config.CertFile, Config.KeyFile)
			if err != nil {
				logError(err.Error())
			}
		} else {
			for {
				conn, err := listener.Accept()
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
		if Config.CertFile != "" && Config.KeyFile != "" {
			err = srv.ServeTLS(listener, Config.CertFile, Config.KeyFile)
		} else {
			err = srv.Serve(listener)
		}
		if err != nil {
			logError(err.Error())
		}
	}
	running = false

	if discover.IsClient() || discover.IsServer() {
		logInfo("stopping discover")
		discover.Stop()
	}
	logInfo("stopping router")
	rh.Stop()

	logInfo("waiting router")
	rh.Wait()
	logInfo("waiting discover")
	discover.Wait()
	serviceInfo.remove()

	logInfo("stopped")
	if as != nil {
		as.stopChan <- true
	}
	return
}

func IsRunning() bool {
	return running
}
