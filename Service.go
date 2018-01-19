package s

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/ssgo/base"
	"golang.org/x/net/http2"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Arr []interface{}
type Map map[string]interface{}

var recordLogs = true

var config = struct {
	Listen           string
	RwTimeout        int
	KeepaliveTimeout int
	CallTimeout      int
	LogFile          string
	NoLogHeaders     string
	CertFile         string
	KeyFile          string
	Discover         string
	DiscoverPrefix   string
	AccessTokens     map[string]uint
	App              string
	Weight           uint
	Calls map[string]struct {
		AccessToken string
		Timeout     int
		HttpVersion int
	}
}{}
var noLogHeaders = map[string]bool{}

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
	clientPool  *ClientPool
}

func (as *AsyncServer) Stop() {
	if as.listener != nil {
		as.listener.Close()
	}
	if as.stopChan != nil {
		<-as.stopChan
	}
}
func (as *AsyncServer) Get(path string, headers ... string) *Result {
	return as.Post(path, nil, headers...)
}
func (as *AsyncServer) Post(path string, data interface{}, headers ... string) *Result {
	if as.clientPool == nil {
		if as.httpVersion == 1 {
			as.clientPool = GetClient1()
		} else {
			as.clientPool = GetClient()
		}
	}
	return as.clientPool.Do(fmt.Sprintf("http://%s%s", as.Addr, path), data, headers...)
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
	return as
}

func initConfig() {
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
		config.KeepaliveTimeout = 10000
	}

	if config.CallTimeout <= 0 {
		config.CallTimeout = 5000
	}

	if config.Discover == "" {
		config.Discover = "discover:15"
	}

	if config.Weight <= 0 {
		config.Weight = 1
	}

	if config.NoLogHeaders == "" {
		config.NoLogHeaders = "Accept,Accept-Encoding,Accept-Language,Cache-Control,Pragma,Connection,Upgrade-Insecure-Requests"
	}
	for _, k := range strings.Split(config.NoLogHeaders, ",") {
		noLogHeaders[strings.TrimSpace(k)] = true
	}
}

func start(httpVersion int, as *AsyncServer) error {
	initConfig()

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
		fmt.Println("**** 1111SERVER	", err)
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
			if an.IP.IsGlobalUnicast() {
				ip = an.IP
			}
		}
	}
	serverAddr := fmt.Sprintf("%s:%d", ip.String(), port)

	if startDiscover(serverAddr) == false {
		log.Printf("SERVER	Failed to start discover")
	}

	log.Printf("SERVER	%s	Started", serverAddr)

	if as != nil {
		as.Addr = serverAddr
		as.startChan <- true
	}
	if httpVersion == 2 {
		srv.TLSConfig = &tls.Config{NextProtos: []string{"http/2", "http/1.1"}}
		s2 := &http2.Server{
			IdleTimeout: 1 * time.Minute,
		}
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

	log.Printf("SERVER	%s	Stopping", serverAddr)
	stopDiscover()
	rh.Stop()

	rh.Wait()
	waitDiscover()
	log.Printf("SERVER	%s	Stopped", serverAddr)
	if as != nil {
		as.stopChan <- true
	}
	return nil
}

//func EnableLogs(enabled bool) {
//	recordLogs = enabled
//}

type routeHandler struct {
	webRequestingNum int64
	wsConns          map[string]*websocket.Conn
	// TODO 记录正在处理的请求数量，连接中的WS数量，在关闭服务时能优雅的结束
}

func (rh *routeHandler) Stop() {
	for _, conn := range rh.wsConns {
		conn.Close()
	}
}

func (rh *routeHandler) Wait() {
	for i := 0; i < 25; i++ {
		if rh.webRequestingNum == 0 && len(rh.wsConns) == 0 {
			break
		}
		time.Sleep(time.Millisecond * 200)
	}
}

func (rh *routeHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	startTime := time.Now()
	// 获取路径
	requestPath := request.RequestURI
	pos := strings.LastIndex(requestPath, "?")
	if pos != -1 {
		requestPath = requestPath[0:pos]
	}
	request.RequestURI = requestPath
	args := make(map[string]interface{})

	// 先看缓存中是否有
	s := webServices[requestPath]
	var ws *websocketServiceType = nil
	if s == nil {
		ws = websocketServices[requestPath]
	}

	// 未匹配到缓存，尝试匹配新的Service
	if s == nil && ws == nil {
		for _, tmpS := range regexWebServices {
			finds := tmpS.pathMatcher.FindAllStringSubmatch(requestPath, 20)
			if len(finds) > 0 {
				foundArgs := finds[0]
				for i := 1; i < len(foundArgs); i++ {
					//log.Println("  >>>>", tmpS.pathArgs[i-1], foundArgs[i])
					args[tmpS.pathArgs[i-1]] = foundArgs[i]
					s = tmpS
				}
				break
			}
		}
	}

	// 未匹配到缓存和Service，尝试匹配新的WebsocketService
	if s == nil && ws == nil {
		for _, tmpS := range regexWebsocketServices {
			finds := tmpS.pathMatcher.FindAllStringSubmatch(requestPath, 20)
			if len(finds) > 0 {
				foundArgs := finds[0]
				for i := 1; i < len(foundArgs); i++ {
					args[tmpS.pathArgs[i-1]] = foundArgs[i]
					ws = tmpS
				}
				break
			}
		}
	}

	// 全都未匹配，输出404
	if s == nil && ws == nil {
		response.WriteHeader(404)
		return
	}

	// GET POST
	request.ParseForm()
	for k, v := range request.Form {
		if len(v) > 1 {
			args[k] = v
		} else {
			args[k] = v[0]
		}
	}

	// POST JSON
	bodyBytes, _ := ioutil.ReadAll(request.Body)
	defer request.Body.Close()
	if len(bodyBytes) > 1 && bodyBytes[0] == 123 {
		json.Unmarshal(bodyBytes, &args)
	}

	if request.Header.Get("S-Unique-Id") == "" {
		request.Header.Set("S-Unique-Id", base.UniqueId())
	}

	// Headers，未来可以优化日志记录，最近访问过的头部信息可省略
	headers := make(map[string]string)
	for k, v := range request.Header {
		if noLogHeaders[k] {
			continue
		}
		if len(v) > 1 {
			headers[k] = strings.Join(v, ", ")
		} else {
			headers[k] = v[0]
		}
	}

	if webAuthChecker != nil {
		var al uint = 0
		if ws != nil {
			al = ws.authLevel
		} else if s != nil {
			al = s.authLevel
		}
		if al > 0 && webAuthChecker(al, &request.RequestURI, &args, request) == false {
			usedTime := float32(time.Now().UnixNano()-startTime.UnixNano()) / 1e6
			byteArgs, _ := json.Marshal(args)
			byteHeaders, _ := json.Marshal(headers)
			log.Printf("REJECT	%s	%s	%s	%s	%.6f	%s	%s	%d	%s", request.RemoteAddr, request.Host, request.Method, request.RequestURI, usedTime, string(byteArgs), string(byteHeaders), al, request.Proto)
			response.WriteHeader(403)
			return
		}
	}

	// 处理 Websocket
	if ws != nil {
		doWebsocketService(ws, request, &response, &args, &headers, &startTime)
	} else if s != nil {
		doWebService(s, request, &response, &args, &headers, &startTime)
	}
}
