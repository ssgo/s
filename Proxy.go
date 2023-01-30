package s

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/ssgo/httpclient"
	"github.com/ssgo/log"
	"github.com/ssgo/standard"
	"github.com/ssgo/u"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ssgo/discover"
)

type proxyInfo struct {
	matcher   *regexp.Regexp
	authLevel int
	fromPath  string
	toApp     string
	toPath    string
}

var proxies = make(map[string]*proxyInfo, 0)
var regexProxies = make([]*proxyInfo, 0)
var proxyBy func(*Request) (int, *string, *string, map[string]string)

// 跳转
func SetProxyBy(by func(request *Request) (authLevel int, toApp, toPath *string, headers map[string]string)) {
	//forceDiscoverClient = true // 代理模式强制启动 Discover Client
	proxyBy = by
}

// 代理
func Proxy(authLevel int, path string, toApp, toPath string) {
	p := &proxyInfo{authLevel: authLevel, fromPath: path, toApp: toApp, toPath: toPath}
	if strings.Contains(path, "(") {
		matcher, err := regexp.Compile("^" + path + "$")
		if err != nil {
			logError(err.Error(), "expr", "^"+path+"$")
		} else {
			p.matcher = matcher
			regexProxies = append(regexProxies, p)
		}
	}
	if p.matcher == nil {
		proxies[path] = p
	}
}

// 查找 Proxy
func findProxy(request *Request) (int, *string, *string) {
	var requestPath string
	var queryString string
	pos := strings.LastIndex(request.RequestURI, "?")
	if pos != -1 {
		requestPath = request.RequestURI[0:pos]
		queryString = request.RequestURI[pos:]
	} else {
		requestPath = request.RequestURI
	}
	pi := proxies[requestPath]
	if pi != nil {
		toPath := pi.toPath + queryString
		return pi.authLevel, &pi.toApp, &toPath
	}
	if len(regexProxies) > 0 {
		for _, pi := range regexProxies {
			finds := pi.matcher.FindAllStringSubmatch(requestPath, 20)
			if len(finds) > 0 {
				toPath := pi.toPath
				for i, partValue := range finds[0] {
					toPath = strings.Replace(toPath, fmt.Sprintf("$%d", i), partValue, 10)
				}
				if queryString != "" {
					toPath += queryString
				}
				return pi.authLevel, &pi.toApp, &toPath
			}
		}
	}
	return 0, nil, nil
}

// ProxyBy
func processProxy(request *Request, response *Response, startTime *time.Time, requestLogger *log.Logger) (finished bool) {
	authLevel, proxyToApp, proxyToPath := findProxy(request)
	var proxyHeaders map[string]string
	if proxyBy != nil && (proxyToApp == nil || proxyToPath == nil || *proxyToApp == "" || *proxyToPath == "") {
		authLevel, proxyToApp, proxyToPath, proxyHeaders = proxyBy(request)
	}
	if proxyToApp == nil || proxyToPath == nil || *proxyToApp == "" || *proxyToPath == "" {
		return false
	}

	if pass, _ := webAuthChecker(authLevel, requestLogger, &request.RequestURI, nil, request, response, nil); pass == false {
		if !response.changed {
			response.WriteHeader(403)
		}
		writeLog(requestLogger, "REJECT", nil, 0, request.Request, response, nil, startTime, authLevel, nil)
		return
	}

	//if recordLogs {
	//	//log.Printf("PROXY	%s	%s	%s	%s	%s	%s", getRealIp(request), request.Host, request.Method, request.RequestURI, *proxyToApp, *proxyToPath)
	//}

	requestHeaders := make([]string, 0)
	if proxyHeaders != nil {
		for k, v := range proxyHeaders {
			requestHeaders = append(requestHeaders, k, v)
		}
	}
	//if appConf.Headers != nil {
	//	for k, v := range appConf.Headers {
	//		requestHeaders = append(requestHeaders, k, v)
	//	}
	//}

	// 续传 X-...
	for _, h := range standard.DiscoverRelayHeaders {
		if request.Header.Get(h) != "" {
			requestHeaders = append(requestHeaders, h, request.Header.Get(h))
		}
	}

	//// 真实的用户IP，通过 X-Real-IP 续传
	//requestHeaders = append(requestHeaders, standard.DiscoverHeaderClientIp, getRealIp(request))
	//
	//// 客户端IP列表，通过 X-Forwarded-For 接力续传
	//requestHeaders = append(requestHeaders, standard.DiscoverHeaderForwardedFor, request.Header.Get(standard.DiscoverHeaderForwardedFor)+u.StringIf(request.Header.Get(standard.DiscoverHeaderForwardedFor) == "", "", ", ")+request.RemoteAddr[0:strings.IndexByte(request.RemoteAddr, ':')])
	//
	//// 客户唯一编号，通过 X-Client-ID 续传
	//if request.Header.Get(standard.DiscoverHeaderClientId) != "" {
	//	requestHeaders = append(requestHeaders, standard.DiscoverHeaderClientId, request.Header.Get(standard.DiscoverHeaderClientId))
	//}
	//
	//// 会话唯一编号，通过 X-Session-ID 续传
	//if request.Header.Get(standard.DiscoverHeaderSessionId) != "" {
	//	requestHeaders = append(requestHeaders, standard.DiscoverHeaderSessionId, request.Header.Get(standard.DiscoverHeaderSessionId))
	//}
	//
	//// 请求唯一编号，通过 X-Request-ID 续传
	//requestId := request.Header.Get(standard.DiscoverHeaderRequestId)
	//if requestId == "" {
	//	requestId = u.UniqueId()
	//	request.Header.Set(standard.DiscoverHeaderRequestId, requestId)
	//}
	//requestHeaders = append(requestHeaders, standard.DiscoverHeaderRequestId, requestId)
	//
	//// 真实用户请求的Host，通过 X-Host 续传
	//host := request.Header.Get(standard.DiscoverHeaderHost)
	//if host == "" {
	//	host = request.Host
	//	request.Header.Set(standard.DiscoverHeaderHost, host)
	//}
	//requestHeaders = append(requestHeaders, standard.DiscoverHeaderHost, host)

	outLen := 0
	//var outBytes []byte

	// 实现简单的负载均衡
	app := *proxyToApp
	if app[0] == '[' && app[len(app)-1] == ']' {
		apps := make([]string, 0)
		err := json.Unmarshal([]byte(*proxyToApp), &app)
		if err == nil {
			n := u.GlobalRand1.Intn(len(apps))
			app = apps[n]
		}
	}

	if !strings.Contains(app, "://") {
		// 代理请求到app，注册新的Call，并重启订阅
		if discover.Config.Calls == nil {
			discover.Config.Calls = make(map[string]string)
		}
		if discover.Config.Calls[app] == "" {
			//log.Printf("PROXY	add app	%s	for	%s	%s	%s", app, request.Host, request.Method, request.RequestURI)
			requestLogger.Info("add app on proxy", Map{
				"app":    app,
				"ip":     getRealIp(request.Request),
				"host":   request.Host,
				"method": request.Method,
				"uri":    request.RequestURI,
			})
			discover.AddExternalApp(app, u.String(Config.RewriteTimeout))
			discover.Restart()
		}
	}
	//fmt.Println("    ^^^^%%%%%%%", app, discover.Config.Calls[app])
	// 处理短连接 Proxy
	if request.Header.Get("Upgrade") == "websocket" {
		outLen = proxyWebsocketRequest(app, *proxyToPath, request, response, requestHeaders, requestLogger)
	} else {
		proxyWebRequest(app, *proxyToPath, request, response, requestHeaders, requestLogger)
		//outLen = proxyWebRequestReverse(app, *proxyToPath, request, response, requestHeaders, appConf.HttpVersion)
	}

	writeLog(requestLogger, "PROXY", nil, outLen, request.Request, response, nil, startTime, 0, Map{
		"toApp":        app,
		"toPath":       proxyToPath,
		"proxyHeaders": proxyHeaders,
	})
	return true
}

var httpClientPool *httpclient.ClientPool = nil

func proxyWebRequest(app, path string, request *Request, response *Response, requestHeaders []string, requestLogger *log.Logger) {
	//var bodyBytes []byte = nil
	//if request.Body != nil {
	//	bodyBytes, _ = ioutil.ReadAll(request.Body)
	//	request.Body.Close()
	//}

	var r *httpclient.Result
	if !strings.Contains(app, "://") {
		caller := &discover.Caller{Request: request.Request, NoBody: true}
		r = caller.Do(request.Method, app, path, request.Body, requestHeaders...)
	} else {
		if httpClientPool == nil {
			httpClientPool = httpclient.GetClient(time.Duration(Config.RewriteTimeout) * time.Millisecond)
			httpClientPool.NoBody = true
		}
		r = httpClientPool.DoByRequest(request.Request, request.Method, app+path, request.Body, requestHeaders...)
	}

	//var statusCode int
	//var outBytes []byte
	if r.Error == nil && r.Response != nil {
		//statusCode = r.Response.StatusCode
		//outBytes = r.Bytes()
		for k, v := range r.Response.Header {
			response.Header().Set(k, v[0])
		}
		response.WriteHeader(r.Response.StatusCode)
		outLen, err := io.Copy(response.writer, r.Response.Body)
		if err != nil {
			if strings.Contains(err.Error(), "stream closed") {
				requestLogger.Warning(err.Error(), "app", app, "path", path, "responseSize", outLen)
				response.outLen = int(outLen)
			} else {
				requestLogger.Error(err.Error(), "app", app, "path", path)
				response.WriteHeader(500)
				response.outLen = len(err.Error())
				n, err := response.Write([]byte(err.Error()))
				if err != nil {
					requestLogger.Error(err.Error(), "wrote", n)
				}
				//statusCode = 500
				//outBytes = []byte(r.Error.Error())
			}
		} else {
			response.outLen = int(outLen)
		}
	} else {
		//statusCode = 500
		//outBytes = []byte(r.Error.Error())
		requestLogger.Error("no response when do proxy", "app", app, "path", path)
		response.WriteHeader(500)
		n, err := response.Write([]byte(r.Error.Error()))
		if err != nil {
			requestLogger.Error(err.Error(), "wrote", n)
		}
		response.outLen = len(r.Error.Error())
	}
}

var updater = websocket.Upgrader{}

func proxyWebsocketRequest(app, path string, request *Request, response *Response, requestHeaders []string, requestLogger *log.Logger) int {
	srcConn, err := updater.Upgrade(response.writer, request.Request, nil)
	if err != nil {
		requestLogger.Error(err.Error(), Map{
			"app":    app,
			"path":   path,
			"ip":     getRealIp(request.Request),
			"method": request.Method,
			"host":   request.Host,
			"uri":    request.RequestURI,
		})
		//log.Printf("PROXY	Upgrade	%s", err.Error())
		return 0
	}
	defer func() {
		_ = srcConn.Close()
	}()

	isHttp := strings.Contains(app, "://")

	appClient := discover.AppClient{}
	for {
		addr := ""
		if !isHttp {
			node := appClient.NextWithNode(app, "", request.Request)
			if node == nil {
				break
			}
			// 请求节点
			node.UsedTimes++
			addr = node.Addr
		} else {
			addr = strings.SplitN(app, "://", 2)[1]
		}

		scheme := "ws"
		//if appConf.WithSSL {
		//	scheme += "s"
		//}
		parsedUrl, err := url.Parse(fmt.Sprintf("%s://%s%s", scheme, addr, path))
		if err != nil {
			requestLogger.Error(err.Error(), Map{
				"app":    app,
				"path":   path,
				"ip":     getRealIp(request.Request),
				"method": request.Method,
				"host":   request.Host,
				"uri":    request.RequestURI,
				"url":    fmt.Sprintf("%s://%s%s", scheme, addr, path),
			})
			//log.Printf("PROXY	parsing websocket address	%s", err.Error())
			return 0
		}

		sendHeader := http.Header{}
		for k, vv := range request.Header {
			if k != "Connection" && k != "Upgrade" && !strings.Contains(k, "Sec-Websocket-") {
				sendHeader.Set(k, vv[0])
			}
		}
		sendHeader.Set("Host", request.Host)

		for i := 1; i < len(requestHeaders); i += 2 {
			sendHeader.Set(requestHeaders[i-1], requestHeaders[i])
		}

		//if httpVersion != 1 {
		//	rp.Transport = &http2.Transport{
		//		AllowHTTP: true,
		//		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
		//			return net.Dial(network, addr)
		//		},
		//	}
		//}

		//dialer := websocket.Dialer{NetDial: func(network, addr string) (conn net.Conn, err error) {
		//	return net.Dial(network, addr)
		//}, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, Proxy: func(request *http.Request) (u2 *url.URL, err error) {
		//	u1 := *request.URL
		//	u1.Scheme = "http"
		//	return &u1, nil
		//}}

		dialer := websocket.Dialer{}
		dstConn, dstResponse, err := dialer.Dial(parsedUrl.String(), sendHeader)
		if err != nil {
			requestLogger.Error(err.Error(), Map{
				"app":    app,
				"path":   path,
				"ip":     getRealIp(request.Request),
				"method": request.Method,
				"host":   request.Host,
				"uri":    request.RequestURI,
				"url":    parsedUrl.String(),
			})
			//log.Printf("PROXY	opening client websocket connection	%s", err.Error())
			if !isHttp {
				continue
			} else {
				break
			}
		}
		if dstResponse.StatusCode == 502 || dstResponse.StatusCode == 503 || dstResponse.StatusCode == 504 {
			_ = dstConn.Close()
			if !isHttp {
				continue
			} else {
				break
			}
		}

		waits := make(chan bool, 2)
		totalOutLen := 0
		go func() {
			for {
				mt, message, err := dstConn.ReadMessage()
				if err != nil {
					if !strings.Contains(err.Error(), "websocket: close ") {
						requestLogger.Error(err.Error(), Map{
							"app":    app,
							"path":   path,
							"ip":     getRealIp(request.Request),
							"method": request.Method,
							"host":   request.Host,
							"uri":    request.RequestURI,
							"url":    parsedUrl.String(),
						})
						//log.Print("PROXY	WS Error	reading message from the client websocket	", err)
					}
					break
				}
				totalOutLen += len(message)
				err = srcConn.WriteMessage(mt, message)
				if err != nil {
					requestLogger.Error(err.Error(), Map{
						"app":    app,
						"path":   path,
						"ip":     getRealIp(request.Request),
						"method": request.Method,
						"host":   request.Host,
						"uri":    request.RequestURI,
						"url":    parsedUrl.String(),
					})
					//log.Print("PROXY	WS Error	writing message to the server websocket	", err)
					break
				}
			}
			waits <- true
			_ = dstConn.Close()
		}()

		go func() {
			for {
				mt, message, err := srcConn.ReadMessage()
				if err != nil {
					if !strings.Contains(err.Error(), "websocket: close ") {
						requestLogger.Error(err.Error(), Map{
							"app":    app,
							"path":   path,
							"ip":     getRealIp(request.Request),
							"method": request.Method,
							"host":   request.Host,
							"uri":    request.RequestURI,
							"url":    parsedUrl.String(),
						})
						//log.Print("PROXY	WS Error	reading message from the server websocket	", err)
					}
					break
				}
				err = dstConn.WriteMessage(mt, message)
				if err != nil {
					requestLogger.Error(err.Error(), Map{
						"app":    app,
						"path":   path,
						"ip":     getRealIp(request.Request),
						"method": request.Method,
						"host":   request.Host,
						"uri":    request.RequestURI,
						"url":    parsedUrl.String(),
					})
					//log.Print("PROXY	WS Error	writing message to the server websocket	", err)
					break
				}
			}
			waits <- true
			_ = srcConn.Close()
		}()

		<-waits
		<-waits
		return totalOutLen
	}

	return 0
}

//func proxyWebRequestReverse(app, path string, request *http.Request, response *Response, requestHeaders []string, httpVersion int) int {
//	appClient := discover.AppClient{}
//	var node *discover.NodeInfo
//	for {
//		node = appClient.NextWithNode(app, "", request)
//		if node == nil {
//			break
//		}
//
//		// 请求节点
//		node.UsedTimes++
//
//		rp := &httputil.ReverseProxy{Director: func(req *http.Request) {
//			req.URL.Scheme = u.StringIf(request.URL.Scheme == "", "http", request.URL.Scheme)
//			if request.TLS != nil {
//				req.URL.Scheme += "s"
//			}
//			req.URL.Host = node.Addr
//			req.URL.Path = path
//			for k, vv := range request.Header {
//				for _, v := range vv {
//					req.Header.Add(k, v)
//				}
//			}
//			for i := 1; i < len(requestHeaders); i += 2 {
//				if requestHeaders[i-1] == "Host" {
//					req.Host = requestHeaders[i]
//				} else {
//					req.Header.Set(requestHeaders[i-1], requestHeaders[i])
//				}
//			}
//		}}
//		if httpVersion != 1 {
//			rp.Transport = &http2.Transport{
//				AllowHTTP: true,
//				DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
//					return net.Dial(network, addr)
//				},
//			}
//		}
//		response.ProxyHeader = &http.Header{}
//		rp.ServeHTTP(response, request)
//		if response.status != 502 && response.status != 503 && response.status != 504 {
//			break
//		}
//	}
//
//	return response.outLen
//}
