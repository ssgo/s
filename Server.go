package s

import (
	"errors"
	"fmt"
	"github.com/ssgo/base"
	"github.com/ssgo/discover"
	"github.com/ssgo/httpclient"
	"golang.org/x/net/http2"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Arr = []interface{}

type Map = map[string]interface{}

var recordLogs = true
var inited = false
var running = false

type configInfo struct {
	Listen            string
	HttpVersion       int
	RwTimeout         int
	KeepaliveTimeout  int
	CallTimeout       int
	LogFile           string
	NoLogGets         bool
	NoLogHeaders      string
	LogResponseSize   int
	Compress          bool
	XUniqueId         string
	XForwardedForName string
	XRealIpName       string
	CertFile          string
	KeyFile           string
	Registry          string
	RegistryCalls     string
	RegistryPrefix    string
	AccessTokens      map[string]*uint
	App               string
	Weight            uint
	AppAllows         []string
	Calls             map[string]*Call
	CallRetryTimes    uint8
}

var config = configInfo{}

type Call struct {
	AccessToken string
	Host        string
	Timeout     int
	HttpVersion int
	WithSSL     bool
}

var noLogHeaders = map[string]bool{}
var serverAddr string
var checker func(request *http.Request) bool

func SetChecker(ck func(request *http.Request) bool) {
	checker = ck
}

func defaultChecker(request *http.Request, response http.ResponseWriter) {
	if request.Header.Get("Pid") != strconv.Itoa(serviceInfo.pid) {
		response.WriteHeader(591)
		return
	}

	var ok bool
	if checker != nil {
		ok = running && checker(request)
	} else {
		ok = running
	}

	if ok {
		response.WriteHeader(299)
	} else {
		if !running {
			response.WriteHeader(592)
		} else {
			response.WriteHeader(593)
		}
	}
}

func GetConfig() configInfo {
	return config
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
	r := as.clientPool.Do(method, fmt.Sprintf("%s://%s%s", base.StringIf(config.CertFile != "" && config.KeyFile != "", "https", "http"), as.Addr, path), data, headers...)
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
	if as.httpVersion == 1 || config.CertFile != "" {
		as.clientPool = httpclient.GetClient(time.Duration(config.CallTimeout) * time.Millisecond)
	} else {
		as.clientPool = httpclient.GetClientH2C(time.Duration(config.CallTimeout) * time.Millisecond)
	}
	return as
}

func Init() {
	inited = true
	base.LoadConfig("service", &config)

	log.SetFlags(log.Ldate | log.Lmicroseconds)
	if config.LogFile != "" {
		f, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			log.SetOutput(f)
		} else {
			log.SetOutput(os.Stdout)
			log.Print("ERROR	", err)
		}
		recordLogs = config.LogFile != os.DevNull
	} else {
		log.SetOutput(os.Stdout)
	}

	if config.KeepaliveTimeout <= 0 {
		config.KeepaliveTimeout = 15000
	}

	if config.CallTimeout <= 0 {
		config.CallTimeout = 10000
	}

	if config.Registry == "" {
		config.Registry = "discover:15"
	}
	if config.RegistryCalls == "" {
		config.RegistryCalls = "discover:15"
	}
	if config.CallRetryTimes <= 0 {
		config.CallRetryTimes = 10
	}

	if config.App != "" && config.App[0] == '_' {
		log.Print("ERROR	", config.App, " is a not available name")
		config.App = ""
	}

	if config.Weight <= 0 {
		config.Weight = 1
	}

	if config.LogResponseSize == 0 {
		config.LogResponseSize = 2048
	}

	if config.XUniqueId == "" {
		config.XUniqueId = "S-Unique-Id"
	}

	if config.XForwardedForName == "" {
		config.XForwardedForName = "S-Forwarded-For"
	}

	if config.XRealIpName == "" {
		config.XRealIpName = "S-Real-Ip"
	}

	if config.NoLogHeaders == "" {
		config.NoLogHeaders = "Accept,Accept-Encoding,Accept-Language,Cache-Control,Pragma,Connection,Upgrade-Insecure-Requests"
	}
	for _, k := range strings.Split(config.NoLogHeaders, ",") {
		noLogHeaders[strings.TrimSpace(k)] = true
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

	if config.HttpVersion == 1 || config.HttpVersion == 2 {
		httpVersion = config.HttpVersion
	}

	log.Printf("SERVER	[%s]	Starting...", config.Listen)

	rh := routeHandler{}
	srv := &http.Server{
		Addr:    config.Listen,
		Handler: &rh,
	}

	if config.RwTimeout > 0 {
		srv.ReadTimeout = time.Duration(config.RwTimeout) * time.Millisecond
		srv.ReadHeaderTimeout = time.Duration(config.RwTimeout) * time.Millisecond
		srv.WriteTimeout = time.Duration(config.RwTimeout) * time.Millisecond
	}

	if config.KeepaliveTimeout > 0 {
		srv.IdleTimeout = time.Duration(config.KeepaliveTimeout) * time.Millisecond
	}

	listener, err := net.Listen("tcp", config.Listen)
	if as != nil {
		as.listener = listener
	}
	if err != nil {
		log.Print("SERVER	", err)
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
				break
			}
		}
	}
	serverAddr = fmt.Sprintf("%s:%d", ip.String(), port)

	dconf := discover.Config{
		Registry:          config.Registry,
		RegistryPrefix:    config.RegistryPrefix,
		RegistryCalls:     config.RegistryCalls,
		App:               config.App,
		Weight:            config.Weight,
		CallRetryTimes:    config.CallRetryTimes,
		XUniqueId:         config.XUniqueId,
		XForwardedForName: config.XForwardedForName,
		XRealIpName:       config.XRealIpName,
		CallTimeout:       config.CallTimeout,
	}
	calls := map[string]*discover.CallInfo{}
	for k, v := range config.Calls {
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
		log.Printf("SERVER	failed to start discover")
		listener.Close()
		return errors.New("failed to start discover")
	}

	// 信息记录到 pid file
	serviceInfo.pid = os.Getpid()
	serviceInfo.httpVersion = httpVersion
	if config.CertFile != "" && config.KeyFile != "" {
		serviceInfo.baseUrl = "https://" + serverAddr
	} else {
		serviceInfo.baseUrl = "http://" + serverAddr
	}
	serviceInfo.save()

	Restful(0, "HEAD", "/__CHECK__", defaultChecker)

	log.Printf("SERVER	%s	Started", serverAddr)

	if as != nil {
		as.Addr = serverAddr
		as.startChan <- true
	}
	if httpVersion == 2 {
		//srv.TLSConfig = &tls.Config{NextProtos: []string{"http/2", "http/1.1"}}
		s2 := &http2.Server{}
		err := http2.ConfigureServer(srv, s2)
		if err != nil {
			log.Print("SERVER	", err)
			return err
		}

		if config.CertFile != "" && config.KeyFile != "" {
			srv.ServeTLS(listener, config.CertFile, config.KeyFile)
		} else {
			for {
				conn, err := listener.Accept()
				if err != nil {
					if strings.Contains(err.Error(), "use of closed network connection") {
						break
					}
					log.Print("SERVER	", err)
					continue
				}
				go s2.ServeConn(conn, &http2.ServeConnOpts{BaseConfig: srv})
			}
		}
	} else {
		if config.CertFile != "" && config.KeyFile != "" {
			srv.ServeTLS(listener, config.CertFile, config.KeyFile)
		} else {
			srv.Serve(listener)
		}
	}
	running = false

	if discover.IsClient() || discover.IsServer() {
		log.Printf("SERVER	%s	Stopping Discover", serverAddr)
		discover.Stop()
	}
	log.Printf("SERVER	%s	Stopping Router", serverAddr)
	rh.Stop()

	log.Printf("SERVER	%s	Waitting Router", serverAddr)
	rh.Wait()
	if discover.IsClient() {
		log.Printf("SERVER	%s	Waitting Discover", serverAddr)
		discover.Wait()
	}
	serviceInfo.remove()

	log.Printf("SERVER	%s	Stopped", serverAddr)
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
