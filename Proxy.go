package s

import (
	"crypto/tls"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/ssgo/base"
	"github.com/ssgo/discover"
	"golang.org/x/net/http2"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type proxyInfo struct {
	matcher  *regexp.Regexp
	fromPath string
	toApp    string
	toPath   string
}

var proxies = make(map[string]*proxyInfo, 0)
var regexProxies = make([]*proxyInfo, 0)
var proxyBy func(*http.Request) (*string, *string, *map[string]string)

// 跳转
func SetProxyBy(by func(request *http.Request) (toApp, toPath *string, headers *map[string]string)) {
	//forceDiscoverClient = true // 代理模式强制启动 Discover Client
	proxyBy = by
}

// 代理
func Proxy(path string, toApp, toPath string) {
	p := &proxyInfo{fromPath: path, toApp: toApp, toPath: toPath}
	if strings.Contains(path, "(") {
		matcher, err := regexp.Compile("^" + path + "$")
		if err != nil {
			log.Printf("PROXY	Compile	%s", err.Error())
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
func findProxy(request *http.Request) (*string, *string) {
	var requestPath string
	var queryString string
	pos := strings.LastIndex(request.RequestURI, "?")
	if pos != -1 {
		requestPath = request.RequestURI[0:pos]
		queryString = requestPath[pos:]
	} else {
		requestPath = request.RequestURI
	}
	pi := proxies[requestPath]
	if pi != nil {
		return &pi.toApp, &pi.toPath
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
				return &pi.toApp, &toPath
			}
		}
	}
	return nil, nil
}

// ProxyBy
func processProxy(request *http.Request, response *Response, logHeaders *map[string]string, startTime *time.Time) (finished bool) {
	proxyToApp, proxyToPath := findProxy(request)
	var proxyHeaders *map[string]string
	if proxyBy != nil && (proxyToApp == nil || proxyToPath == nil || *proxyToApp == "" || *proxyToPath == "") {
		proxyToApp, proxyToPath, proxyHeaders = proxyBy(request)
	}
	if proxyToApp == nil || proxyToPath == nil || *proxyToApp == "" || *proxyToPath == "" {
		return false
	}

	if recordLogs {
		log.Printf("PROXY	%s	%s	%s	%s	%s	%s", getRealIp(request), request.Host, request.Method, request.RequestURI, *proxyToApp, *proxyToPath)
	}

	// 注册新的Call，并重启订阅
	if config.Calls[*proxyToApp] == nil {
		log.Printf("PROXY	add app	%s	for	%s	%s	%s", *proxyToApp, request.Host, request.Method, request.RequestURI)
		config.Calls[*proxyToApp] = &Call{HttpVersion: 2}
		discover.AddExternalApp(*proxyToApp, discover.CallInfo{})
		discover.Restart()
	}

	appConf := config.Calls[*proxyToApp]
	requestHeaders := make([]string, 0)
	if proxyHeaders != nil {
		for k, v := range *proxyHeaders {
			requestHeaders = append(requestHeaders, k, v)
		}
	}
	requestHeaders = append(requestHeaders, "Host", request.Host)
	if appConf != nil && appConf.AccessToken != "" {
		requestHeaders = append(requestHeaders, "Access-Token", appConf.AccessToken)
	}
	uniqueId := request.Header.Get(config.XUniqueId)
	if request.Header.Get(config.XUniqueId) != "" {
		requestHeaders = append(requestHeaders, config.XUniqueId, uniqueId)
	}
	requestHeaders = append(requestHeaders, config.XRealIpName, getRealIp(request))
	requestHeaders = append(requestHeaders, config.XForwardedForName, request.Header.Get(config.XForwardedForName)+base.StringIf(request.Header.Get(config.XForwardedForName) == "", "", ", ")+request.RemoteAddr[0:strings.IndexByte(request.RemoteAddr, ':')])

	outLen := 0
	var outBytes []byte

	// 处理短连接 Proxy
	if request.Header.Get("Upgrade") == "websocket" {
		outLen = proxyWebsocketRequest(*proxyToApp, *proxyToPath, request, response, requestHeaders, appConf)
	} else {
		outBytes = proxyWebRequest(*proxyToApp, *proxyToPath, request, response, requestHeaders)
		outLen = len(outBytes)
		//outLen = proxyWebRequestReverse(*proxyToApp, *proxyToPath, request, response, requestHeaders, appConf.HttpVersion)
	}

	if recordLogs {
		writeLog("REDIRECT", nil, outLen, request, response, nil, logHeaders, startTime, 0)
	}
	return true
}

func proxyWebRequest(app, path string, request *http.Request, response *Response, requestHeaders []string) []byte {
	var bodyBytes []byte = nil
	if request.Body != nil {
		bodyBytes, _ = ioutil.ReadAll(request.Body)
		request.Body.Close()
	}
	caller := &discover.Caller{Request: request}
	r := caller.Do(request.Method, app, path, bodyBytes, requestHeaders...)

	var statusCode int
	var outBytes []byte
	if r.Error == nil && r.Response != nil {
		statusCode = r.Response.StatusCode
		outBytes = r.Bytes()
		for k, v := range r.Response.Header {
			response.Header().Set(k, v[0])
		}
	} else {
		statusCode = 500
		outBytes = []byte(r.Error.Error())
	}

	response.WriteHeader(statusCode)
	response.Write(outBytes)
	return outBytes
}

var updater = websocket.Upgrader{}

func proxyWebsocketRequest(app, path string, request *http.Request, response *Response, requestHeaders []string, appConf *Call) int {
	srcConn, err := updater.Upgrade(response.writer, request, nil)
	if err != nil {
		log.Printf("PROXY	Upgrade	%s", err.Error())
		return 0
	}
	defer srcConn.Close()

	appClient := discover.AppClient{}
	var node *discover.NodeInfo
	for {
		node = appClient.NextWithNode(app, "", request)
		if node == nil {
			break
		}

		// 请求节点
		node.UsedTimes++

		scheme := "ws"
		if appConf.WithSSL {
			scheme += "s"
		}
		u, err := url.Parse(fmt.Sprintf("%s://%s%s", scheme, node.Addr, path))
		if err != nil {
			log.Printf("PROXY	parsing websocket address	%s", err.Error())
			return 0
		}

		sendHeader := http.Header{}
		for k, vv := range request.Header {
			if k != "Connection" && k != "Upgrade" && !strings.Contains(k, "Sec-Websocket-") {
				for _, v := range vv {
					sendHeader.Add(k, v)
				}
			}
		}
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

		dialer := websocket.Dialer{}
		dstConn, dstResponse, err := dialer.Dial(u.String(), sendHeader)
		if err != nil {
			log.Printf("PROXY	opening client websocket connection	%s", err.Error())
			continue
		}
		if dstResponse.StatusCode == 502 || dstResponse.StatusCode == 503 || dstResponse.StatusCode == 504 {
			dstConn.Close()
			continue
		}

		waits := make(chan bool, 2)
		totalOutLen := 0
		go func() {
			defer dstConn.Close()
			for {
				mt, message, err := dstConn.ReadMessage()
				if err != nil {
					if !strings.Contains(err.Error(), "websocket: close ") {
						log.Print("PROXY	WS Error	reading message from the client websocket	", err)
					}
					break
				}
				totalOutLen += len(message)
				err = srcConn.WriteMessage(mt, message)
				if err != nil {
					log.Print("PROXY	WS Error	writing message to the server websocket	", err)
					break
				}
			}
			waits <- true
		}()

		go func() {
			defer srcConn.Close()
			for {
				mt, message, err := srcConn.ReadMessage()
				if err != nil {
					if !strings.Contains(err.Error(), "websocket: close ") {
						log.Print("PROXY	WS Error	reading message from the server websocket	", err)
					}
					break
				}
				err = dstConn.WriteMessage(mt, message)
				if err != nil {
					log.Print("PROXY	WS Error	writing message to the server websocket	", err)
					break
				}
			}
			waits <- true
		}()

		<-waits
		return totalOutLen
	}

	return 0
}

func proxyWebRequestReverse(app, path string, request *http.Request, response *Response, requestHeaders []string, httpVersion int) int {
	appClient := discover.AppClient{}
	var node *discover.NodeInfo
	for {
		node = appClient.NextWithNode(app, "", request)
		if node == nil {
			break
		}

		// 请求节点
		node.UsedTimes++

		rp := &httputil.ReverseProxy{Director: func(req *http.Request) {
			req.URL.Scheme = base.StringIf(request.URL.Scheme == "", "http", request.URL.Scheme)
			if request.TLS != nil {
				req.URL.Scheme += "s"
			}
			req.URL.Host = node.Addr
			req.URL.Path = path
			for k, vv := range request.Header {
				for _, v := range vv {
					req.Header.Add(k, v)
				}
			}
			for i := 1; i < len(requestHeaders); i += 2 {
				if requestHeaders[i-1] == "Host" {
					req.Host = requestHeaders[i]
				} else {
					req.Header.Set(requestHeaders[i-1], requestHeaders[i])
				}
			}
		}}
		if httpVersion != 1 {
			rp.Transport = &http2.Transport{
				AllowHTTP: true,
				DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
					return net.Dial(network, addr)
				},
			}
		}
		response.ProxyHeader = &http.Header{}
		rp.ServeHTTP(response, request)
		if response.status != 502 && response.status != 503 && response.status != 504 {
			break
		}
	}

	return response.outLen
}
