package s

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ssgo/discover"
	"github.com/ssgo/log"
	"github.com/ssgo/standard"
	"github.com/ssgo/u"
	"golang.org/x/net/websocket"
)

//type Request struct {
//	http.Request
//	injects map[reflect.Type]any
//}
//
//// 设置一个生命周期在 Request 中的对象，请求中可以使用对象类型注入参数方便调用
//func (request *Request) SetInject(obj any) {
//	if request.injects == nil {
//		request.injects = map[reflect.Type]any{}
//	}
//	request.injects[reflect.TypeOf(obj)] = obj
//}
//
//// 获取本生命周期中指定类型的 Session 对象
//func (request *Request) GetInject(dataType reflect.Type) any {
//	if request.injects == nil {
//		return nil
//	}
//	return request.injects[dataType]
//}

//type Uploader struct {
//	request *http.Request
//}

type UploadFile struct {
	fileHeader *multipart.FileHeader
	Filename   string
	Header     textproto.MIMEHeader
	Size       int64
}

//func (uploader *Uploader) Fields() []string {
//	fields := make([]string, 0)
//	if uploader.request.MultipartForm != nil {
//		for field := range uploader.request.MultipartForm.File {
//			fields = append(fields, field)
//		}
//	}
//	return fields
//}
//
//func (uploader *Uploader) File(field string) *UploadFile {
//	uploadFiles := uploader.Files(field)
//	if len(uploadFiles) > 0 {
//		return uploadFiles[0]
//	}
//	return nil
//}
//
//func (uploader *Uploader) Files(field string) []*UploadFile {
//	uploadFiles := make([]*UploadFile, 0)
//	if uploader.request.MultipartForm != nil {
//		if fileHeaders := uploader.request.MultipartForm.File[field]; fileHeaders != nil {
//			for _, fileHeader := range fileHeaders {
//				uploadFiles = append(uploadFiles, &UploadFile{
//					fileHeader: fileHeader,
//					Filename:   fileHeader.Filename,
//					Header:     fileHeader.Header,
//					Size:       fileHeader.Size,
//				})
//			}
//		}
//	}
//	return uploadFiles
//}

func (uploadFile *UploadFile) Open() (multipart.File, error) {
	return uploadFile.fileHeader.Open()
}

func (uploadFile *UploadFile) Save(filename string) error {
	u.CheckPath(filename)
	if dstFile, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600); err == nil {
		if srcFile, err := uploadFile.fileHeader.Open(); err == nil {
			defer srcFile.Close()
			io.Copy(dstFile, srcFile)
			return nil
		} else {
			return err
		}
	} else {
		return err
	}
}

func (uploadFile *UploadFile) Content() ([]byte, error) {
	if file, err := uploadFile.fileHeader.Open(); err == nil {
		buf, err := io.ReadAll(file)
		if err == nil {
			return buf, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

type Request struct {
	*http.Request
	contextValues map[string]any
	Id            string
}

func (request *Request) ResetPath(path string) {
	request.RequestURI = path
	if newUrl, err := url.Parse(path); err == nil {
		request.URL = newUrl
	}
}

func (request *Request) Set(key string, value any) {
	request.contextValues[key] = value
}

func (request *Request) Get(key string) any {
	return request.contextValues[key]
}

type Response struct {
	Id            string
	Writer        http.ResponseWriter
	status        int
	outLen        int
	changed       bool
	headerWritten bool
	dontLog200    bool
	dontLogArgs   []string
	ProxyHeader   *http.Header
}

type Logger = log.Logger

func MakeUrl(request *http.Request, path string) string {
	return fmt.Sprint(request.Header.Get(standard.DiscoverHeaderScheme), "://", request.Header.Get(standard.DiscoverHeaderHost), path)
}
func (request *Request) MakeUrl(path string) string {
	return MakeUrl(request.Request, path)
}

func (response *Response) Header() http.Header {
	//if response.headerWritten {
	//	return nil
	//}
	response.changed = true
	if response.ProxyHeader != nil {
		return *response.ProxyHeader
	}
	return response.Writer.Header()
}
func (response *Response) Write(bytes []byte) (int, error) {
	response.checkWriteHeader()
	response.changed = true
	response.outLen += len(bytes)
	if response.ProxyHeader != nil {
		response.copyProxyHeader()
	}
	return response.Writer.Write(bytes)
}
func (response *Response) WriteString(s string) (int, error) {
	return response.Write([]byte(s))
}
func (response *Response) WriteHeader(code int) {
	response.changed = true
	response.status = code
	if response.ProxyHeader != nil && (response.status == 502 || response.status == 503 || response.status == 504) {
		return
	}
	if response.ProxyHeader != nil {
		response.copyProxyHeader()
	}
}
func (response *Response) checkWriteHeader() {
	if !response.headerWritten {
		response.headerWritten = true
		if response.status != 200 {
			response.Writer.WriteHeader(response.status)
		}
		return
	}
}

func (response *Response) Flush() {
	if flusher, ok := response.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}
func (response *Response) FlushString(s string) (int, error) {
	n, err := response.WriteString(s)
	if err == nil {
		response.Flush()
	}
	return n, err
}

func (response *Response) copyProxyHeader() {
	src := *response.ProxyHeader
	dst := response.Writer.Header()
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
	response.ProxyHeader = nil
}

func (response *Response) DontLog200() {
	response.dontLog200 = true
}

func (response *Response) DontLogArg(arg string) {
	response.dontLogArgs = append(response.dontLogArgs, arg)
}

func (response *Response) Location(location string) {
	response.WriteHeader(302)
	response.Header().Set("Location", location)
}

func (response *Response) SendFile(contentType, filename string) {
	response.Header().Set("Content-Type", contentType)
	if mf := u.ReadFileFromMemory(filename); mf != nil {
		response.Write(mf.Data)
		return
	}
	if fd, err := os.Open(filename); err == nil {
		defer fd.Close()
		_, _ = io.Copy(response, fd)
	}
}

func (response *Response) DownloadFile(contentType, filename string, data any) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	response.Header().Set("Content-Type", contentType)

	if filename != "" {
		response.Header().Set("Content-Disposition", "attachment; filename="+filename)
	}

	var outBytes []byte = nil
	var reader io.Reader = nil
	if v, ok := data.([]byte); ok {
		outBytes = v
	} else if v, ok := data.(string); ok {
		outBytes = []byte(v)
	} else if v, ok := data.(io.Reader); ok {
		reader = v
	} else {
		outBytes = []byte(u.Json(data))
	}

	if outBytes != nil {
		response.Header().Set("Content-Length", u.String(len(outBytes)))
		response.Write(outBytes)
	} else if reader != nil {
		io.Copy(response, reader)
	}
}

func GetDomainWithScope(request *http.Request, scope string) string {
	host := request.Header.Get(standard.DiscoverHeaderHost)
	if scope == "topDomain" {
		//domain := strings.SplitN(host, ":", 2)[0]
		domain := host
		domainParts := strings.Split(domain, ".")
		if len(domainParts) >= 2 {
			domain = domainParts[len(domainParts)-2] + "." + domainParts[len(domainParts)-1]
		}
		return domain
	} else {
		return host
	}
}

type routeHandler struct {
	webRequestingNum int64
	wsConns          map[string]*websocket.Conn
	// TODO 记录正在处理的请求数量，连接中的WS数量，在关闭服务时能优雅的结束
}

func (rh *routeHandler) Stop() {
	for _, conn := range rh.wsConns {
		_ = conn.Close()
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

func xHeader(headerName string, request *http.Request, maker func() string) string {
	h := request.Header.Get(headerName)
	if h == "" {
		h = maker()
		request.Header.Set(headerName, h)
	}
	return h
}

func getLogHeaders(request *http.Request) map[string]string {
	// Headers，未来可以优化日志记录，最近访问过的头部信息可省略
	logHeaders := make(map[string]string)
	for k, v := range request.Header {
		if noLogHeaders[strings.ToLower(k)] {
			continue
		}
		if len(v) > 1 {
			logHeaders[k] = strings.Join(v, ", ")
		} else {
			logHeaders[k] = v[0]
		}
	}
	return logHeaders
}

func parseRequestURI(request *http.Request, args *map[string]any) {
	if strings.Index(request.RequestURI, request.URL.Path) == -1 && strings.LastIndex(request.RequestURI, "?") != -1 {
		requestUrl, reqErr := url.Parse(request.RequestURI)
		if reqErr == nil {
			queryStringArr, reqErr := url.ParseQuery(requestUrl.RawQuery)
			if reqErr == nil && len(queryStringArr) > 0 {
				for paramName, paramValue := range queryStringArr {
					if len(paramValue) < 1 {
						continue
					}
					if len(paramValue) > 1 {
						(*args)[paramName] = paramValue
					} else {
						(*args)[paramName] = paramValue[0]
					}
				}
			}
		}
	}
}

func parseService(request *http.Request, host, requestPath string, args *map[string]any) (*webServiceType, *websocketServiceType) {
	var s *webServiceType
	var ws *websocketServiceType

	webServicesLock.RLock()
	s = webServices[fmt.Sprint(host, request.Method, requestPath)]
	//fmt.Println(111, fmt.Sprint(host, request.Method, requestPath), s)
	if s == nil {
		//fmt.Println(222, fmt.Sprint(host, requestPath), s)
		s = webServices[fmt.Sprint(host, requestPath)]
		if s == nil {
			port := ":80"
			if request.TLS != nil {
				port = ":443"
			}
			if strings.ContainsRune(host, ':') {
				port = ":" + strings.SplitN(host, ":", 2)[1]
			}
			//fmt.Println(333, fmt.Sprint(port, request.Method, requestPath), s)
			s = webServices[fmt.Sprint(port, request.Method, requestPath)]
			if s == nil {
				//fmt.Println(444, fmt.Sprint(request.Method, requestPath), s)
				s = webServices[fmt.Sprint(request.Method, requestPath)]
				if s == nil {
					s = webServices[requestPath]
				}
			}
		}
	}
	webServicesLock.RUnlock()

	if s == nil {
		websocketServicesLock.RLock()
		ws = websocketServices[fmt.Sprint(host, requestPath)]
		if ws == nil {
			ws = websocketServices[requestPath]
		}
		websocketServicesLock.RUnlock()
	}

	// 未匹配到缓存，尝试匹配新的 Service
	if s == nil && ws == nil {
		//for _, tmpS := range regexWebServices {
		maxRegexWebServicesKey := len(regexWebServices) - 1
		for i := maxRegexWebServicesKey; i >= 0; i-- {
			tmpS := regexWebServices[i]
			if tmpS.method != "" && tmpS.method != request.Method {
				continue
			}
			if tmpS.options.Host != "" && tmpS.options.Host != request.Host {
				if tmpS.options.Host[0] == ':' {
					// 判断是否匹配端口
					optionHostPort := "80"
					if request.TLS != nil {
						optionHostPort = "443"
					}
					if strings.Contains(request.Host, ":") {
						optionHostPort = strings.SplitN(request.Host, ":", 2)[1]
					}
					requestHostPort := tmpS.options.Host[1:]
					if optionHostPort != requestHostPort {
						continue
					}
				} else {
					continue
				}
			}
			finds := tmpS.pathMatcher.FindAllStringSubmatch(requestPath, 20)
			if len(finds) > 0 {
				foundArgs := finds[0]
				for i := 1; i < len(foundArgs); i++ {
					unescaped, err := url.QueryUnescape(foundArgs[i])
					if err == nil {
						(*args)[tmpS.pathArgs[i-1]] = unescaped
					} else {
						(*args)[tmpS.pathArgs[i-1]] = foundArgs[i]
					}
					s = tmpS
				}
				break
			}
		}
	}

	// 未匹配到缓存和Service，尝试匹配新的WebsocketService
	if s == nil && ws == nil {
		//for _, tmpS := range regexWebsocketServices {
		for i := len(regexWebsocketServices) - 1; i >= 0; i-- {
			tmpS := regexWebsocketServices[i]
			if tmpS.options.Host != "" && tmpS.options.Host != request.Host {
				if tmpS.options.Host[0] == ':' {
					// 判断是否匹配端口
					optionHostPort := "80"
					if request.TLS != nil {
						optionHostPort = "443"
					}
					if strings.Contains(request.Host, ":") {
						optionHostPort = strings.SplitN(request.Host, ":", 2)[1]
					}
					requestHostPort := tmpS.options.Host[1:]
					if optionHostPort != requestHostPort {
						continue
					}
				} else {
					continue
				}
			}
			finds := tmpS.pathMatcher.FindAllStringSubmatch(requestPath, 20)
			if len(finds) > 0 {
				foundArgs := finds[0]
				for i := 1; i < len(foundArgs); i++ {
					(*args)[tmpS.pathArgs[i-1]] = foundArgs[i]
					ws = tmpS
				}
				break
			}
		}
	}
	return s, ws
}

func (rh *routeHandler) ServeHTTP(writer http.ResponseWriter, httpRequest *http.Request) {
	startTime := time.Now()

	var tc *TimeCounter
	if Config.StatisticTime {
		tc = StartTimeCounter()
		defer func() {
			timeStatistic.Push(tc)
			//log.DefaultLogger.Info("time count", "request", request.RequestURI, "count", tc.Sprint())
		}()
	}

	var request = &Request{Request: httpRequest, contextValues: map[string]any{}}
	var response = &Response{Writer: writer, status: 200}
	defer response.checkWriteHeader()
	var sessionObject any = nil

	headerSetSessionId := ""
	headerSetDeviceId := ""
	requestId := ""
	host := ""
	//var logHeaders map[string]string
	if !Config.Fast {
		// 在没有 X-Request-ID 的情况下忽略 X-Real-IP
		if request.Header.Get(standard.DiscoverHeaderRequestId) == "" && !Config.AcceptXRealIpWithoutRequestId {
			request.Header.Del(standard.DiscoverHeaderClientIp)
		}

		// 产生 X-Request-ID
		requestId = xHeader(standard.DiscoverHeaderRequestId, request.Request, UniqueId)

		// 真实的用户IP，通过 X-Real-IP 续传
		xHeader(standard.DiscoverHeaderClientIp, request.Request, func() string {
			return request.RemoteAddr[0:strings.IndexByte(request.RemoteAddr, ':')]
		})

		// 真实用户请求的Host，通过 X-Host 续传
		host = xHeader(standard.DiscoverHeaderHost, request.Request, func() string {
			return request.Host
		})

		// 真实用户请求的Scheme，通过 X-Scheme 续传
		xHeader(standard.DiscoverHeaderScheme, request.Request, func() string {
			return u.StringIf(request.TLS == nil, "http", "https")
		})

		// UA
		xHeader(standard.DiscoverHeaderUserAgent, request.Request, func() string {
			return request.Header.Get("User-Agent")
		})

		// SessionId
		if usedSessionIdKey != "" {
			// 优先从 Header 中读取
			// sessionId := GetSessionId(request, response)
			sessionId := request.Header.Get(usedSessionIdKey)
			if sessionId == "" && !Config.SessionWithoutCookie {
				if ck, err := request.Cookie(usedSessionIdKey); err == nil {
					sessionId = ck.Value
					if sessionId != "" {
						headerSetSessionId = sessionId
					}
				}
			}
			if sessionId == "" {
				sessionId = request.Header.Get(standard.DiscoverHeaderSessionId)
			}

			if sessionId == "" {
				// 自动生成 SessionId
				if sessionIdMaker != nil {
					sessionId = sessionIdMaker()
				} else {
					sessionId = UniqueId20()
				}
				if !Config.SessionWithoutCookie {
					cookie := http.Cookie{
						Name:     usedSessionIdKey,
						Value:    sessionId,
						Path:     "/",
						HttpOnly: true,
					}
					if Config.CookieScope != "host" {
						cookie.Domain = GetDomainWithScope(request.Request, Config.CookieScope)
					}
					http.SetCookie(response, &cookie)
				}
				headerSetSessionId = sessionId
				// response.Header().Set(usedSessionIdKey, sessionId)
			}
			// 为了在服务间调用时续传 SessionId
			request.Header.Set(standard.DiscoverHeaderSessionId, sessionId)
		}

		// DeviceId
		if usedDeviceIdKey != "" {
			// 优先从 Header 中读取
			deviceId := request.Header.Get(usedDeviceIdKey)
			if deviceId == "" && !Config.SessionWithoutCookie {
				// 尝试从 Cookie 中读取
				if cookie, err := request.Cookie(usedDeviceIdKey); err == nil {
					deviceId = cookie.Value
					if deviceId != "" {
						headerSetDeviceId = deviceId
					}
				}
			}
			if deviceId == "" {
				deviceId = request.Header.Get(standard.DiscoverHeaderDeviceId)
			}
			if deviceId == "" {
				// 自动生成 DeviceId
				deviceId = UniqueId20()
				if !Config.SessionWithoutCookie {
					cookie := http.Cookie{
						Name:     usedDeviceIdKey,
						Value:    deviceId,
						Path:     "/",
						Expires:  time.Now().AddDate(10, 0, 0),
						HttpOnly: true,
					}
					if Config.CookieScope != "host" {
						cookie.Domain = GetDomainWithScope(request.Request, Config.CookieScope)
					}

					http.SetCookie(response, &cookie)
				}
				headerSetDeviceId = deviceId
				// response.Header().Set(usedDeviceIdKey, deviceId)
			}
			// 为了在服务间调用时续传 DeviceId
			request.Header.Set(standard.DiscoverHeaderDeviceId, deviceId)
		}

		// ClientAppName、ClientAppVersion
		if usedClientAppKey != "" {
			// 为了在服务间调用时续传 ClientAppName、ClientAppVersion
			request.Header.Set(standard.DiscoverHeaderClientAppName, request.Header.Get(usedClientAppKey+"Name"))
			request.Header.Set(standard.DiscoverHeaderClientAppVersion, request.Header.Get(usedClientAppKey+"Version"))
		}

		//fmt.Println(u.JsonP(logHeaders))

		if Config.StatisticTime {
			tc.Add("Make Headers")
		}
	} else {
		requestId = UniqueId()
		host = request.Host
	}

	request.Id = requestId
	response.Id = requestId
	requestLogger := log.New(requestId)
	if Config.Fast {
		if Config.StatisticTime {
			tc.Add("Pre")
		}
	}

	if httpRequest.Method == "PRI" {
		writer.WriteHeader(200)
		writer.Header().Set("Upgrade", "h2c")
		writer.Header().Set("Connection", "Upgrade")
		writer.Header().Set("Content-Length", "0")
		writer.Header().Set("Content-Type", "text/plain")
		writeLog(requestLogger, "PRI", nil, 0, request.Request, response, nil, &startTime, 0, nil)
		return
	}

	// 处理 Rewrite，如果是外部转发，直接结束请求
	if processRewrite(request, response, &startTime, requestLogger) {
		return
	}

	if Config.StatisticTime {
		tc.Add("Check Rewrite")
	}

	//var requestPath string
	//pos := strings.LastIndex(request.RequestURI, "?")
	//if pos != -1 {
	//	requestPath = request.RequestURI[0:pos]
	//} else {
	//	requestPath = request.RequestURI
	//}
	requestPath := request.URL.Path
	// 处理静态文件
	processStaticOK, staticFile := processStatic(requestPath, request, response, &startTime, requestLogger)
	if processStaticOK {
		return
	}

	if Config.StatisticTime {
		tc.Add("Check Static")
	}

	// 处理 ProxyBy
	if processProxy(request, response, &startTime, requestLogger) {
		return
	}

	if Config.StatisticTime {
		tc.Add("Check Proxy")
	}

	if headerSetSessionId != "" {
		response.Header().Set(usedSessionIdKey, headerSetSessionId)
	}
	if headerSetDeviceId != "" {
		response.Header().Set(usedDeviceIdKey, headerSetDeviceId)
	}

	args := make(map[string]any)

	// 先看缓存中是否有 Service
	//var s *webServiceType
	//var ws *websocketServiceType
	//fmt.Println(request.Host, request.Method, requestPath)
	//fmt.Println(u.JsonP(webServices))
	s, ws := parseService(request.Request, host, requestPath, &args)
	if Config.StatisticTime {
		tc.Add("Find Service")
	}

	// 判定是rewrite
	// rewrite问号后的参数不能被request.Form解析 解析问号后的参数
	parseRequestURI(request.Request, &args)

	// GET POST
	err := request.ParseForm()
	if err != nil {
		logError(err.Error())
	} else {
		reqForm := request.Form
		for k, v := range reqForm {
			if len(v) > 1 {
				args[k] = v
			} else {
				args[k] = v[0]
			}
		}
	}

	noBody := false
	noLog200 := false
	if s != nil {
		noBody = s.options.NoBody
		noLog200 = s.options.NoLog200
	} else if ws != nil {
		noBody = ws.options.NoBody
		noLog200 = ws.options.NoLog200
	}

	if noLog200 {
		response.dontLog200 = true
	}

	// POST
	if request.Body != nil && !noBody {
		contentType := request.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "application/json") {
			bodyBytes, _ := io.ReadAll(request.Body)
			_ = request.Body.Close()
			if len(bodyBytes) > 0 {
				var err error
				if bodyBytes[0] == 123 {
					err = json.Unmarshal(bodyBytes, &args)
				} else {
					arg := new(any)
					err = json.Unmarshal(bodyBytes, arg)
					args["request"] = arg
				}
				if err != nil {
					ServerLogger.Error(err.Error())
					response.WriteHeader(400)
					writeLog(requestLogger, "FAIL", nil, 0, request.Request, response, args, &startTime, 0, nil)
					return
				}
			}
		} else if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
			bodyBytes, _ := io.ReadAll(request.Body)
			_ = request.Body.Close()
			argsBody, err := url.ParseQuery(string(bodyBytes))
			if err == nil && len(argsBody) > 0 {
				for aKey, aValue := range argsBody {
					aKey = strings.ReplaceAll(aKey, "[]", "")
					if len(aValue) > 1 {
						args[aKey] = aValue
					} else {
						args[aKey] = aValue[0]
					}
				}
			}
		} else if strings.HasPrefix(contentType, "multipart/form-data") {
			err := request.ParseMultipartForm(Config.MaxUploadSize)
			if err == nil {
				defer request.MultipartForm.RemoveAll()
				for aKey, aValue := range request.MultipartForm.Value {
					aKey = strings.ReplaceAll(aKey, "[]", "")
					if len(aValue) > 1 {
						args[aKey] = aValue
					} else if len(aValue) == 1 {
						args[aKey] = aValue[0]
					}
				}
				for aKey, aValue := range request.MultipartForm.File {
					aKey = strings.ReplaceAll(aKey, "[]", "")
					uploads := make([]UploadFile, len(aValue))
					for i := len(aValue) - 1; i >= 0; i-- {
						fileHeader := aValue[i]
						uploads[i] = UploadFile{
							fileHeader: fileHeader,
							Filename:   fileHeader.Filename,
							Header:     fileHeader.Header,
							Size:       fileHeader.Size,
						}
					}
					if len(aValue) > 1 {
						args[aKey] = uploads
					} else if len(aValue) == 1 {
						args[aKey] = uploads[0]
					}
				}
			}
		}
	}

	if Config.StatisticTime {
		tc.Add("Make Args")
	}

	// 前置过滤器
	var result any = nil
	prevRequestURI := request.RequestURI
	for _, filter := range inFilters {
		result = filter(&args, request, response, requestLogger)
		if result != nil {
			break
		}
	}

	// 重定向
	if prevRequestURI != request.RequestURI {
		requestPath = request.URL.Path
		s, ws = parseService(request.Request, host, requestPath, &args)
		parseRequestURI(request.Request, &args)
	}

	if Config.StatisticTime {
		tc.Add("In Filter")
	}

	var authLevel = 0
	var options *WebServiceOptions
	if ws != nil {
		authLevel = ws.authLevel
	} else if s != nil {
		options = &s.options
		authLevel = s.authLevel
	}

	defer func() {
		if err := recover(); err != nil {
			var out any
			if errorHandle != nil {
				out = errorHandle(err, request, response)
			} else {
				response.WriteHeader(ResponseCodePanicError)
				out = ""
			}

			logError(u.String(err))
			writeLog(requestLogger, "PANIC", out, response.outLen, request.Request, response, args, &startTime, authLevel, Map{
				"error": u.String(err),
			})
		}

		//if sessionObjects[request] != nil {
		//	delete(sessionObjects, request)
		//}
	}()

	// 全都未匹配，输出404（在前置过滤器之后再判断404）
	if s == nil && ws == nil {
		response.WriteHeader(404)
		if requestPath != "/favicon.ico" {
			writeLog(requestLogger, "FAIL", nil, 0, request.Request, response, args, &startTime, 0, Map{
				"file": staticFile,
			})
		}
		return
	}

	if result == nil {
		// 之前未产生结果，进行验证
		ac := webAuthCheckers[authLevel]
		if ac == nil {
			ac = webAuthChecker
		}
		pass, object := ac(authLevel, requestLogger, &request.RequestURI, args, request, response, options)
		if pass == false {
			//usedTime := float32(time.Now().UnixNano()-startTime.UnixNano()) / 1e6
			//byteArgs, _ := json.Marshal(args)
			//byteHeaders, _ := json.Marshal(logHeaders)
			//log.Printf("REJECT	%s	%s	%s	%s	%.6f	%s	%s	%d	%s", request.RemoteAddr, request.Host, request.Method, request.RequestURI, usedTime, string(byteArgs), string(byteHeaders), authLevel, request.Proto)
			response.WriteHeader(403)
			if object == nil && webAuthFailedData != nil {
				object = webAuthFailedData
			}
			if object == nil {
				outLen := 0
				if !response.changed {
					response.checkWriteHeader()
				} else {
					outLen = response.outLen
				}
				writeLog(requestLogger, "REJECT", result, outLen, request.Request, response, args, &startTime, authLevel, nil)
			} else {
				var outData any
				var outLen int
				outBytes, outContentType := makeOutput(object)
				if outContentType != "" {
					response.Header().Set("Content-Type", outContentType)
				}
				n, err := response.Write(outBytes)
				if err != nil {
					logError(err.Error(), "wrote", n)
				}
				outData = object
				outLen = len(outBytes)
				writeLog(requestLogger, "ACCESS", outData, outLen, request.Request, response, args, &startTime, authLevel, nil)
			}
			return
		} else {
			sessionObject = object
		}
	}

	if Config.StatisticTime {
		tc.Add("Auth Check")
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
		doWebsocketService(ws, request, response, authLevel, args, &startTime, requestLogger, sessionObject)
	} else if s != nil || result != nil {
		result = doWebService(s, request, response, args, result, requestLogger, sessionObject)
		//logName = "ACCESS"
		//statusCode = 200
	}
	//}
	if Config.StatisticTime {
		tc.Add("Do Request")
	}

	if response.dontLogArgs != nil && len(response.dontLogArgs) > 0 {
		for _, arg := range response.dontLogArgs {
			delete(args, arg)
		}
	}

	if ws == nil {
		// 后置过滤器
		for _, filter := range outFilters {
			newResult, done := filter(args, request, response, result, requestLogger)
			if newResult != nil {
				result = newResult
			}
			if done {
				break
			}
		}
		if Config.StatisticTime {
			tc.Add("Out Filter")
		}

		// 返回结果
		outBytes, outContentType := makeOutput(result)
		if outContentType != "" {
			response.Header().Set("Content-Type", outContentType)
		}

		isZipOuted := false
		if Config.Compress && len(outBytes) >= Config.CompressMinSize && len(outBytes) <= Config.CompressMaxSize && strings.Contains(request.Header.Get("Accept-Encoding"), "gzip") {
			zipWriter, err := gzip.NewWriterLevel(response, 1)
			if err == nil {
				response.Header().Set("Content-Encoding", "gzip")
				n, err := zipWriter.Write(outBytes)
				if err != nil {
					logError(err.Error(), "wrote", n)
				} else {
					isZipOuted = true
				}
				_ = zipWriter.Close()
			}
		}

		if !isZipOuted {
			n, err := response.Write(outBytes)
			if err != nil {
				logError(err.Error(), "wrote", n)
			}
		}

		// 记录访问日志
		outLen := 0
		if outBytes != nil {
			outLen = len(outBytes)
		}

		if Config.StatisticTime {
			tc.Add("Make Result")
		}

		if requestPath != "/__CHECK__" {
			writeLog(requestLogger, "ACCESS", result, outLen, request.Request, response, args, &startTime, authLevel, nil)
		}

		if Config.StatisticTime {
			tc.Add("Write Log")
		}
	}
}

func makeOutput(result any) (byteResult []byte, contentType string) {
	outType := reflect.TypeOf(result)
	if outType == nil {
		return []byte{}, ""
	}
	for outType.Kind() == reflect.Ptr {
		outType = outType.Elem()
	}
	var outBytes []byte
	if outType.Kind() != reflect.String && (outType.Kind() != reflect.Slice || outType.Elem().Kind() != reflect.Uint8) {
		outBytes = makeBytesResult(result)
		contentType = "application/json; charset=UTF-8"
	} else if outType.Kind() == reflect.String {
		outBytes = []byte(result.(string))
	} else {
		outBytes = result.([]byte)
	}
	return outBytes, contentType
}

//func requireEncryptField(k string) bool {
//	return encryptLogFields[strings.ToLower(strings.Replace(k, "-", "", 3))]
//}
//
//func encryptField(value any) string {
//	v := u.String(value)
//	if len(v) > 12 {
//		return v[0:3] + "***" + v[len(v)-3:]
//	} else if len(v) > 8 {
//		return v[0:2] + "***" + v[len(v)-2:]
//	} else if len(v) > 4 {
//		return v[0:1] + "***" + v[len(v)-1:]
//	} else if len(v) > 1 {
//		return v[0:1] + "*"
//	} else {
//		return "**"
//	}
//}

func writeLog(logger *log.Logger, logName string, result any, outLen int, request *http.Request, response *Response, args map[string]any, startTime *time.Time, authLevel int, extraInfo Map) {
	if Config.NoLogGets && request.Method == "GET" {
		return
	}
	if response.dontLog200 && response.status == 200 {
		return
	}
	usedTime := float32(time.Now().UnixNano()-startTime.UnixNano()) / 1e6
	//if headers != nil {
	//	for k, v := range headers {
	//		if requireEncryptField(k) {
	//			headers[k] = encryptField(v)
	//		}
	//	}
	//}

	outHeaders := make(map[string]string)
	//fmt.Println(111, u.JsonP((*response).Header()), 111)
	for k, v := range (*response).Header() {
		if outLen == 0 && k == "Content-Length" {
			outLen, _ = strconv.Atoi(v[0])
		}
		//if noLogHeaders[strings.ToLower(k)] {
		//	continue
		//}
		if len(v) > 1 {
			outHeaders[k] = strings.Join(v, ", ")
		} else {
			outHeaders[k] = v[0]
		}

		//if requireEncryptField(k) {
		//	outHeaders[k] = encryptField(outHeaders[k])
		//}
	}

	var loggableRequestArgs map[string]any
	if args != nil {
		fixedArgs := makeLogableData(logger, reflect.ValueOf(args), nil, Config.LogInputArrayNum, Config.LogInputFieldSize, 1).Interface()
		if v, ok := fixedArgs.(map[string]any); ok {
			loggableRequestArgs = v
		} else {
			loggableRequestArgs = map[string]any{"data": args}
		}
	} else {
		loggableRequestArgs = map[string]any{}
	}

	fixedResult := ""
	if result != nil {
		resultValue := makeLogableData(logger, reflect.ValueOf(result), noLogOutputFields, Config.LogOutputArrayNum, Config.LogOutputFieldSize, 1)
		if resultValue.IsValid() && resultValue.CanInterface() {
			resultBytes, err := json.Marshal(resultValue.Interface())
			if err == nil {
				if !Config.KeepKeyCase {
					u.FixUpperCase(resultBytes, nil)
				}
				fixedResult = string(resultBytes)
			}
		}
	}

	if extraInfo == nil {
		extraInfo = Map{}
	}
	extraInfo["type"] = logName

	host := request.Header.Get(standard.DiscoverHeaderHost)
	if host == "" {
		host = request.Host
	}

	requestPath := request.URL.Path
	//fmt.Println(u.JsonP(outHeaders) , 123111)

	logger.Request(serverId, discover.Config.App, serverAddr, getRealIp(request), request.Header.Get(standard.DiscoverHeaderFromApp), request.Header.Get(standard.DiscoverHeaderFromNode), request.Header.Get(standard.DiscoverHeaderUserId), request.Header.Get(standard.DiscoverHeaderDeviceId), request.Header.Get(standard.DiscoverHeaderClientAppName), request.Header.Get(standard.DiscoverHeaderClientAppVersion), request.Header.Get(standard.DiscoverHeaderSessionId), request.Header.Get(standard.DiscoverHeaderRequestId), host, u.StringIf(request.TLS == nil, "http", "https"), request.Proto[5:], authLevel, 0, request.Method, requestPath, getLogHeaders(request), loggableRequestArgs, usedTime, response.status, outHeaders, uint(outLen), fixedResult, extraInfo)
}

func makeLogableData(logger *log.Logger, v reflect.Value, notAllows map[string]bool, numArrays int, fieldSize int, level int) reflect.Value {
	v = _makeLogableData(v, notAllows, numArrays, fieldSize, level)
	logger.FixValue(v)
	return v
}

func _makeLogableData(v reflect.Value, notAllows map[string]bool, numArrays int, fieldSize int, level int) reflect.Value {
	t := v.Type()
	if t == nil {
		return reflect.ValueOf(nil)
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}

	if !v.IsValid() {
		return reflect.ValueOf(nil)
	}

	//fmt.Println(strings.Repeat("    ", level), "  ====", t.Kind(), t.Name())
	switch t.Kind() {
	case reflect.Struct:
		v2 := reflect.MakeMap(reflect.TypeOf(Map{}))
		for i := 0; i < v.NumField(); i++ {
			k := t.Field(i).Name
			if k[0] < 'A' || k[0] > 'Z' {
				continue
			}
			if t.Field(i).Tag.Get("log") == "no" {
				continue
			}

			if t.Field(i).Anonymous {
				// 继承的结构
				v3 := _makeLogableData(v.Field(i), notAllows, numArrays, fieldSize, level)
				for _, mk := range v3.MapKeys() {
					v2.SetMapIndex(mk, _makeLogableData(v3.MapIndex(mk), notAllows, numArrays, fieldSize, level+1))
				}
				continue
			}

			//log.DefaultLogger.Info("  ========!!!", "level", level, "k", k)
			if notAllows != nil && notAllows[strings.ToLower(k)] {
				continue
			}
			v2.SetMapIndex(reflect.ValueOf(k), _makeLogableData(v.Field(i), notAllows, numArrays, fieldSize, level+1))
			//fmt.Println("       &&>>>> ", t.Field(i).Name, k, v2.MapIndex(reflect.ValueOf(k)).Type())
			//fmt.Println(strings.Repeat("    ", level), "    ->-> ", t.Field(i).Name, v2.MapIndex(reflect.ValueOf(k)).Type())
		}
		return v2
	case reflect.Map:
		v2 := reflect.MakeMap(reflect.TypeOf(Map{}))
		for _, mk := range v.MapKeys() {
			k := mk.String()
			if notAllows != nil && notAllows[strings.ToLower(k)] {
				continue
			}
			v2.SetMapIndex(mk, _makeLogableData(v.MapIndex(mk), nil, numArrays, fieldSize, level+1))
			//fmt.Println(strings.Repeat("    ", level), "    >>>> ", mk, v2.MapIndex(mk).Type())
		}
		return v2
	case reflect.Slice:
		if numArrays == 0 {
			// 不记录数组内容
			var tStr string
			if t.Elem().Kind() == reflect.Interface && v.Len() > 0 {
				if v.Index(0).CanInterface() {
					tStr = reflect.TypeOf(v.Index(0).Interface()).String()
				} else {
					tStr = "null"
				}
			} else {
				tStr = t.Elem().String()
			}
			return reflect.ValueOf(fmt.Sprintf("%s (%d)", tStr, v.Len()))
		}
		v2 := reflect.MakeSlice(reflect.TypeOf(Arr{}), 0, 0)
		for i := 0; i < v.Len(); i++ {
			if i >= numArrays {
				break
			}
			if v.Index(i).Kind() == reflect.Ptr && !v.Index(i).IsNil() && v.Index(i).IsValid() {
				//fmt.Println(1111,v.Index(i),numArrays, fieldSize, level+1, "|", v2)
				v2 = reflect.Append(v2, _makeLogableData(v.Index(i), nil, numArrays, fieldSize, level+1))
			} else {
				//v2 = reflect.Append(v2, _makeLogableData(v.Index(i), nil, numArrays, fieldSize, level+1))
				v2 = reflect.Append(v2, v.Index(i))
			}
			//fmt.Println(strings.Repeat("    ", level), "    -]-] ", i, v.Index(i).Type())
		}
		return v2
	case reflect.Interface:
		v2 := reflect.ValueOf(v.Interface())
		//fmt.Println(strings.Repeat("    ", level), "        **** Interface", v2.Type())
		if v2.Kind() == reflect.Invalid {
			return reflect.ValueOf(nil)
		}
		if v2.Type().Kind() != reflect.Interface {
			return _makeLogableData(v2, nil, numArrays, fieldSize, level)
		} else {
			return v2
		}
	case reflect.String:
		if fieldSize == 0 || fieldSize > v.Len() {
			return v
		}
		return reflect.ValueOf(v.String()[0:fieldSize])
	default:
		return v
	}
}

func getRealIp(request *http.Request) string {
	ip := request.Header.Get(standard.DiscoverHeaderClientIp)
	if ip == "" {
		ip = request.Header.Get(standard.DiscoverHeaderForwardedFor)
	}
	if ip == "" {
		ip = request.RemoteAddr[0:strings.IndexByte(request.RemoteAddr, ':')]
	}
	return ip
}

func (request *Request) GetRealIp() string {
	return getRealIp(request.Request)
}

/* ================================================================================= */
type GzipResponseWriter struct {
	*Response
	zipWriter *gzip.Writer
}

func (gzw *GzipResponseWriter) Write(b []byte) (int, error) {
	contentLen, err := gzw.zipWriter.Write(b)
	_ = gzw.zipWriter.Flush()
	return contentLen, err
}

func (gzw *GzipResponseWriter) Close() {
	_ = gzw.zipWriter.Close()
}

func NewGzipResponseWriter(w *Response) *GzipResponseWriter {
	w.Header().Set("Content-Encoding", "gzip")

	gz := gzip.NewWriter(w)

	gzw := GzipResponseWriter{
		zipWriter: gz,
		Response:  w,
	}

	return &gzw
}
