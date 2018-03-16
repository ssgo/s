package s

import (
	"compress/gzip"
	"encoding/json"
	"github.com/ssgo/base"
	"golang.org/x/net/websocket"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Response struct {
	writer http.ResponseWriter
	status int
}

func (response *Response) Header() http.Header {
	return response.writer.Header()
}
func (response *Response) Write(bytes []byte) (int, error) {
	return response.writer.Write(bytes)
}
func (response *Response) WriteHeader(code int) {
	response.status = code
	response.writer.WriteHeader(code)
}

//func (response *Response) Hijacker() (net.Conn, *bufio.ReadWriter, error) {
//	h, ok := response.writer.(http.Hijacker)
//	if !ok {
//		return nil, nil, errors.New("bad Hijacker")
//	}
//	return h.Hijack()
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

func (rh *routeHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var myResponse *Response = &Response{writer: writer, status: 200}
	var response http.ResponseWriter = myResponse
	startTime := time.Now()

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

	// 处理 Rewrite，如果是外部转发，直接结束请求
	finished := processRewrite(request, myResponse, &headers, &startTime)
	if finished {
		return
	}

	// 处理 ProxyBy
	finished = processProxy(request, myResponse, &headers, &startTime)
	if finished {
		return
	}

	var requestPath string
	pos := strings.LastIndex(request.RequestURI, "?")
	if pos != -1 {
		requestPath = request.RequestURI[0:pos]
	} else {
		requestPath = request.RequestURI
	}

	// 处理静态文件
	if processStatic(requestPath, request, myResponse, &headers, &startTime) {
		return
	}

	args := make(map[string]interface{})

	// 先看缓存中是否有 Service
	var s *webServiceType
	var ws *websocketServiceType
	s = webServices[request.Method+requestPath]
	if s == nil {
		s = webServices[requestPath]
		if s == nil {
			ws = websocketServices[requestPath]
		}
	}

	// 未匹配到缓存，尝试匹配新的 Service
	if s == nil && ws == nil {
		for _, tmpS := range regexWebServices {
			if tmpS.method != "" && tmpS.method != request.Method {
				continue
			}
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
		if requestPath != "/favicon.ico" {
			writeLog("FAIL", nil, 0, false, request, myResponse, &args, &headers, &startTime, 0)
		}
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
	if request.Body != nil {
		bodyBytes, _ := ioutil.ReadAll(request.Body)
		request.Body.Close()
		if len(bodyBytes) > 1 && bodyBytes[0] == 123 {
			json.Unmarshal(bodyBytes, &args)
		}
	}

	if request.Header.Get("S-Unique-Id") == "" {
		request.Header.Set("S-Unique-Id", base.UniqueId())
		// 在没有 S-Unique-Id 的情况下忽略 X-Real-Ip
		if request.Header.Get(config.XRealIpName) != "" {
			request.Header.Del(config.XRealIpName)
		}
	}

	// SessionId
	if sessionKey != "" {
		if request.Header.Get(sessionKey) == "" {
			var newSessionid string
			if sessionCreator == nil {
				newSessionid = base.UniqueId()
			} else {
				newSessionid = sessionCreator()
			}
			request.Header.Set(sessionKey, newSessionid)
			response.Header().Set(sessionKey, newSessionid)
		}
	}

	// 前置过滤器
	var result interface{} = nil
	for _, filter := range inFilters {
		result = filter(&args, request, &response)
		if result != nil {
			break
		}
	}

	// 身份认证
	var authLevel uint = 0
	if webAuthChecker != nil {
		if ws != nil {
			authLevel = ws.authLevel
		} else if s != nil {
			authLevel = s.authLevel
		}
		if authLevel > 0 && webAuthChecker(authLevel, &request.RequestURI, &args, request) == false {
			//usedTime := float32(time.Now().UnixNano()-startTime.UnixNano()) / 1e6
			//byteArgs, _ := json.Marshal(args)
			//byteHeaders, _ := json.Marshal(headers)
			//log.Printf("REJECT	%s	%s	%s	%s	%.6f	%s	%s	%d	%s", request.RemoteAddr, request.Host, request.Method, request.RequestURI, usedTime, string(byteArgs), string(byteHeaders), authLevel, request.Proto)
			response.WriteHeader(403)
			writeLog("REJECT", nil, 0, false, request, myResponse, &args, &headers, &startTime, authLevel)
			return
		}
	}

	// 处理 Proxy
	//var logName string
	//var statusCode int
	//if proxyToApp != nil {
	//	caller := &Caller{request: request}
	//	r := caller.Do(request.Method, *proxyToApp, *proxyToPath, args, "Host", request.Host)
	//	result = r.Bytes()
	//	statusCode = 500
	//	if r.Error == nil && r.Response != nil {
	//		statusCode = r.Response.StatusCode
	//	}
	//	logName = "PROXY"
	//} else {
	// 处理 Websocket
	if ws != nil && result == nil {
		doWebsocketService(ws, request, myResponse, &args, &headers, &startTime)
	} else if s != nil || result != nil {
		result = doWebService(s, request, &response, &args, &headers, result, &startTime)
		//logName = "ACCESS"
		//statusCode = 200
	}
	//}

	if ws == nil {
		// 后置过滤器
		for _, filter := range outFilters {
			newResult, done := filter(&args, request, &response, result)
			if newResult != nil {
				result = newResult
			}
			if done {
				break
			}
		}

		// 返回结果
		outType := reflect.TypeOf(result)
		if outType.Kind() == reflect.Ptr {
			outType = outType.Elem()
		}
		var outBytes []byte
		isJson := false
		if outType.Kind() != reflect.String && (outType.Kind() != reflect.Slice || outType.Elem().Kind() != reflect.Uint8) {
			outBytes = makeBytesResult(result)
			isJson = true
		} else if outType.Kind() == reflect.String {
			outBytes = []byte(result.(string))
		} else {
			outBytes = result.([]byte)
		}

		isZipOuted := false
		if config.Compress && len(outBytes) > 1024 && strings.Contains(request.Header.Get("Accept-Encoding"), "gzip") {
			zipWriter, err := gzip.NewWriterLevel(response, 1)
			if err == nil {
				response.Header().Set("Content-Encoding", "gzip")
				zipWriter.Write(outBytes)
				zipWriter.Close()
				isZipOuted = true
			}
		}

		if !isZipOuted {
			response.Write(outBytes)
		}

		// 记录访问日志
		if recordLogs {
			outLen := 0
			//if logName != "ACCESS" {
			//	if outBytes != nil {
			//		outLen = len(outBytes)
			//		outBytes = nil
			//	}
			//}

			// /__CHECK__ 不记录日志
			if requestPath != "/__CHECK__" {
				writeLog("ACCESS", outBytes, outLen, isJson, request, myResponse, &args, &headers, &startTime, authLevel)
			}
		}
	}

	if sessionObjects[request] != nil {
		delete(sessionObjects, request)
	}
}

func writeLog(logName string, outBytes []byte, outLen int, isJson bool, request *http.Request, response *Response, args *map[string]interface{}, headers *map[string]string, startTime *time.Time, authLevel uint) {
	usedTime := float32(time.Now().UnixNano()-startTime.UnixNano()) / 1e6
	var byteArgs []byte
	if args != nil {
		byteArgs, _ = json.Marshal(*args)
	}
	var byteHeaders []byte
	if headers != nil {
		at := (*headers)["Access-Token"]
		if at != "" {
			if len(at) > 6 {
				(*headers)["Access-Token"] = at[0:3] + "***" + at[len(at)-3:]
			} else if len(at) > 4 {
				(*headers)["Access-Token"] = at[0:2] + "***" + at[len(at)-2:]
			}
		}
		byteHeaders, _ = json.Marshal(*headers)
	}

	if outLen == 0 && outBytes != nil {
		outLen = len(outBytes)
	}
	outHeaders := make(map[string]string)
	for k, v := range (*response).Header() {
		if outLen == 0 && k == "Content-Length" {
			outLen, _ = strconv.Atoi(v[0])
		}
		if noLogHeaders[k] {
			continue
		}
		if len(v) > 1 {
			outHeaders[k] = strings.Join(v, ", ")
		} else {
			outHeaders[k] = v[0]
		}
	}
	byteOutHeaders, _ := json.Marshal(outHeaders)
	if len(outBytes) > config.LogResponseSize {
		outBytes = outBytes[0:config.LogResponseSize]
	}
	if !isJson {
		makePrintable(outBytes)
	}
	log.Printf("%s	%s	%s	%s	%s	%s	%d	%.6f	%d	%d	%s	%s	%s	%s	%s", logName, getRealIp(request), base.StringIf(config.App != "", config.App, request.Host), serverAddr, request.Method, request.RequestURI, authLevel, usedTime, response.status, outLen, string(byteArgs), string(byteHeaders), string(outBytes), string(byteOutHeaders), request.Proto[5:])
}

func getRealIp(request *http.Request) string {
	return base.StringIf(request.Header.Get(config.XRealIpName) != "", request.Header.Get(config.XRealIpName), request.RemoteAddr[0:strings.IndexByte(request.RemoteAddr, ':')])
}
