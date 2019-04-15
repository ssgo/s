package s

import (
	"errors"
	"fmt"
	"github.com/ssgo/standard"
	golog "log"
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

var recordLogs = true
var inited = false
var running = false

type configInfo struct {
	Listen                        string
	HttpVersion                   int
	RwTimeout                     int
	KeepaliveTimeout              int
	CallTimeout                   int
	LogFile                       string
	LogLevel                      string
	NoLogGets                     bool
	NoLogHeaders                  string
	EncryptLogFields              string
	NoLogInputFields              bool
	LogInputArrayNum              int
	LogOutputFields               string
	LogOutputArrayNum             int
	LogWebsocketAction            bool
	Compress                      bool
	CertFile                      string
	KeyFile                       string
	Registry                      string
	RegistryCalls                 string
	RegistryPrefix                string
	App                           string
	Weight                        uint
	AccessTokens                  map[string]*uint
	AppAllows                     []string
	Calls                         map[string]*Call
	CallRetryTimes                uint8
	AcceptXRealIpWithoutRequestId bool
}

var conf = configInfo{}

//var configedLogLevel log.LevelType

type Call struct {
	AccessToken string
	Host        string
	Timeout     int
	HttpVersion int
	WithSSL     bool
}

var noLogHeaders = map[string]bool{}
var encryptLogFields = map[string]bool{}
var logOutputFields = map[string]bool{}

var serverAddr string
var checker func(request *http.Request) bool

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

func GetConfig() configInfo {
	return conf
}

// 启动HTTP/1.1服务
func Start1() {
	start(1, nil)
}

// 启动HTTP/2服务
func Start() {
	start(2, nil)
}

type AsyncServer struct {
	startChan   chan bool
	stopChan    chan bool
	httpVersion int
	listener    net.Listener
	Addr        string
	clientPool  *httpclient.ClientPool
}

func (as *AsyncServer) Stop() {
	if as.listener != nil {
		as.listener.Close()
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
	r := as.clientPool.Do(method, fmt.Sprintf("%s://%s%s", u.StringIf(conf.CertFile != "" && conf.KeyFile != "", "https", "http"), as.Addr, path), data, headers...)
	if sessionKey != "" && r.Response != nil && r.Response.Header != nil && r.Response.Header.Get(sessionKey) != "" {
		as.clientPool.SetGlobalHeader(sessionKey, r.Response.Header.Get(sessionKey))
	}
	return r
}

func (as *AsyncServer) SetGlobalHeader(k, v string) {
	as.clientPool.SetGlobalHeader(k, v)
}

func AsyncStart() *AsyncServer {
	return asyncStart(2)
}
func AsyncStart1() *AsyncServer {
	return asyncStart(1)
}
func asyncStart(httpVersion int) *AsyncServer {
	as := &AsyncServer{startChan: make(chan bool), stopChan: make(chan bool), httpVersion: httpVersion}
	go start(httpVersion, as)
	<-as.startChan
	if as.httpVersion == 1 || conf.CertFile != "" {
		as.clientPool = httpclient.GetClient(time.Duration(conf.CallTimeout) * time.Millisecond)
	} else {
		as.clientPool = httpclient.GetClientH2C(time.Duration(conf.CallTimeout) * time.Millisecond)
	}
	return as
}

func Init() {
	inited = true
	config.LoadConfig("service", &conf)

	golog.SetFlags(golog.Ldate | golog.Lmicroseconds)
	if conf.LogFile != "" {
		f, err := os.OpenFile(conf.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			golog.SetOutput(f)
		} else {
			golog.SetOutput(os.Stdout)
			log.Error("S", Map{
				"subLogType": "server",
				"type":       "openLogFileFailed",
				"error":      err.Error(),
			})
			//log.Print("ERROR	", err)
		}
		recordLogs = conf.LogFile != os.DevNull
	} else {
		golog.SetOutput(os.Stdout)
	}

	logLevel := strings.ToLower(conf.LogLevel)
	if logLevel == "debug" {
		log.SetLevel(log.DEBUG)
	} else if logLevel == "warning" {
		log.SetLevel(log.WARNING)
	} else if logLevel == "error" {
		log.SetLevel(log.ERROR)
	} else {
		log.SetLevel(log.INFO)
	}

	if conf.KeepaliveTimeout <= 0 {
		conf.KeepaliveTimeout = 15000
	}

	if conf.CallTimeout <= 0 {
		conf.CallTimeout = 10000
	}

	if conf.Registry == "" {
		conf.Registry = "discover:15"
	}
	if conf.RegistryCalls == "" {
		conf.RegistryCalls = "discover:15"
	}
	if conf.CallRetryTimes <= 0 {
		conf.CallRetryTimes = 10
	}

	if conf.App != "" && conf.App[0] == '_' {
		log.Warning("S", Map{
			"warning": "bad app name",
			"app":     conf.App,
		})
		//log.Print("ERROR	", conf.App, " is a not available name")
		conf.App = ""
	}

	if conf.Weight <= 0 {
		conf.Weight = 1
	}

	if conf.NoLogHeaders == "" {
		conf.NoLogHeaders = fmt.Sprint("Accept,Accept-Encoding,Accept-Language,Cache-Control,Pragma,Connection,Upgrade-Insecure-Requests",
			",", standard.DiscoverHeaderClientIp,
			",", standard.DiscoverHeaderForwardedFor,
			",", standard.DiscoverHeaderClientId,
			",", standard.DiscoverHeaderSessionId,
			",", standard.DiscoverHeaderRequestId,
			",", standard.DiscoverHeaderHost,
			",", standard.DiscoverHeaderScheme,
			",", standard.DiscoverHeaderFromApp,
			",", standard.DiscoverHeaderFromNode,
		)
	}
	for _, k := range strings.Split(strings.ToLower(conf.NoLogHeaders), ",") {
		noLogHeaders[strings.TrimSpace(k)] = true
	}

	if conf.EncryptLogFields == "" {
		conf.EncryptLogFields = "password,secure,token,accessToken"
	}
	for _, k := range strings.Split(strings.ToLower(conf.EncryptLogFields), ",") {
		encryptLogFields[strings.TrimSpace(k)] = true
	}

	if conf.LogOutputFields == "" {
		conf.LogOutputFields = "code,message"
	}
	for _, k := range strings.Split(strings.ToLower(conf.LogOutputFields), ",") {
		logOutputFields[strings.TrimSpace(k)] = true
	}

	if conf.LogInputArrayNum <= 0 {
		conf.LogInputArrayNum = 0
	}

	if conf.LogOutputArrayNum <= 0 {
		conf.LogOutputArrayNum = 2
	}
}

func start(httpVersion int, as *AsyncServer) error {
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
				os.Setenv("SERVICE_LISTEN", os.Args[i])
			} else if strings.ContainsRune(os.Args[i], '=') {
				a := strings.SplitN(os.Args[i], "=", 2)
				os.Setenv(a[0], a[1])
			} else {
				os.Setenv("SERVICE_LOGFILE", os.Args[i])
			}
		}
	}
	if !inited {
		Init()
	}

	if conf.HttpVersion == 1 || conf.HttpVersion == 2 {
		httpVersion = conf.HttpVersion
	}

	log.Info("S", Map{
		"info":   "starting",
		"listen": conf.Listen,
	})
	//log.Printf("SERVER	[%s]	Starting...", conf.Listen)

	rh := routeHandler{}
	srv := &http.Server{
		Addr:    conf.Listen,
		Handler: &rh,
	}

	if conf.RwTimeout > 0 {
		srv.ReadTimeout = time.Duration(conf.RwTimeout) * time.Millisecond
		srv.ReadHeaderTimeout = time.Duration(conf.RwTimeout) * time.Millisecond
		srv.WriteTimeout = time.Duration(conf.RwTimeout) * time.Millisecond
	}

	if conf.KeepaliveTimeout > 0 {
		srv.IdleTimeout = time.Duration(conf.KeepaliveTimeout) * time.Millisecond
	}

	listener, err := net.Listen("tcp", conf.Listen)
	if as != nil {
		as.listener = listener
	}
	if err != nil {
		log.Error("S", Map{
			"listen": conf.Listen,
			"error":  "listen failed: " + err.Error(),
		})
		//log.Print("SERVER	", err)
		if as != nil {
			as.startChan <- false
		}
		return err
	}

	closeChan := make(chan os.Signal, 2)
	signal.Notify(closeChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-closeChan
		listener.Close()
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

	dconf := discover.Config{
		Registry:       conf.Registry,
		RegistryPrefix: conf.RegistryPrefix,
		RegistryCalls:  conf.RegistryCalls,
		App:            conf.App,
		Weight:         conf.Weight,
		CallRetryTimes: conf.CallRetryTimes,
		CallTimeout:    conf.CallTimeout,
	}
	calls := map[string]*discover.CallInfo{}
	if conf.Calls == nil {
		conf.Calls = map[string]*Call{}
	}
	for k, v := range conf.Calls {
		call := discover.CallInfo{
			Timeout:     v.Timeout,
			HttpVersion: v.HttpVersion,
			WithSSL:     v.WithSSL,
		}
		call.Headers = map[string]string{}
		if v.AccessToken != "" {
			call.Headers["Access-Token"] = v.AccessToken
		}
		if v.Host != "" {
			call.Headers["Host"] = v.Host
		}
		calls[k] = &call
	}
	dconf.Calls = calls
	if discover.Start(serverAddr, dconf) == false {
		log.Error("S", Map{
			"serverAddr": serverAddr,
			"error":      "discover start failed: " + err.Error(),
		})
		//log.Printf("SERVER	failed to start discover")
		listener.Close()
		return errors.New("failed to start discover")
	}

	// 信息记录到 pid file
	serviceInfo.pid = os.Getpid()
	serviceInfo.httpVersion = httpVersion
	if conf.CertFile != "" && conf.KeyFile != "" {
		serviceInfo.baseUrl = "https://" + serverAddr
	} else {
		serviceInfo.baseUrl = "http://" + serverAddr
	}
	serviceInfo.save()

	Restful(0, "HEAD", "/__CHECK__", defaultChecker)

	log.Info("S", Map{
		"info":       "started",
		"listen":     conf.Listen,
		"serverAddr": serverAddr,
	})
	//log.Printf("SERVER	%s	Started", serverAddr)

	if as != nil {
		as.Addr = serverAddr
		as.startChan <- true
	}
	if httpVersion == 2 {
		//srv.TLSConfig = &tls.Config{NextProtos: []string{"http/2", "http/1.1"}}
		s2 := &http2.Server{}
		err := http2.ConfigureServer(srv, s2)
		if err != nil {
			log.Error("S", Map{
				"listen":     conf.Listen,
				"serverAddr": serverAddr,
				"error":      "start failed:" + err.Error(),
			})
			//log.Print("SERVER	", err)
			return err
		}

		if conf.CertFile != "" && conf.KeyFile != "" {
			srv.ServeTLS(listener, conf.CertFile, conf.KeyFile)
		} else {
			for {
				conn, err := listener.Accept()
				if err != nil {
					if strings.Contains(err.Error(), "use of closed network connection") {
						break
					}
					log.Error("S", Map{
						"listen":     conf.Listen,
						"serverAddr": serverAddr,
						"error":      "listen failed: " + err.Error(),
					})
					//log.Print("SERVER	", err)
					continue
				}
				go s2.ServeConn(conn, &http2.ServeConnOpts{BaseConfig: srv})
			}
		}
	} else {
		if conf.CertFile != "" && conf.KeyFile != "" {
			srv.ServeTLS(listener, conf.CertFile, conf.KeyFile)
		} else {
			srv.Serve(listener)
		}
	}
	running = false

	if discover.IsClient() || discover.IsServer() {
		log.Info("S", Map{
			"info":       "stopping discover",
			"listen":     conf.Listen,
			"serverAddr": serverAddr,
			"isClient":   discover.IsClient(),
			"isServer":   discover.IsServer(),
		})
		//log.Printf("SERVER	%s	Stopping Discover", serverAddr)
		discover.Stop()
	}
	log.Info("S", Map{
		"info":       "stopping router",
		"listen":     conf.Listen,
		"serverAddr": serverAddr,
	})
	//log.Printf("SERVER	%s	Stopping Router", serverAddr)
	rh.Stop()

	log.Info("S", Map{
		"info":       "waiting router",
		"listen":     conf.Listen,
		"serverAddr": serverAddr,
	})
	//log.Printf("SERVER	%s	Waiting Router", serverAddr)
	rh.Wait()
	log.Info("S", Map{
		"info":       "waiting discover",
		"listen":     conf.Listen,
		"serverAddr": serverAddr,
	})
	//log.Printf("SERVER	%s	Waiting Discover", serverAddr)
	discover.Wait()
	serviceInfo.remove()

	log.Info("S", Map{
		"info":       "stopped",
		"listen":     conf.Listen,
		"serverAddr": serverAddr,
	})
	//log.Printf("SERVER	%s	Stopped", serverAddr)
	if as != nil {
		as.stopChan <- true
	}
	return nil
}

func IsRunning() bool {
	return running
}

//func EnableLogs(enabled bool) {
//	recordLogs = enabled
//}
