package s

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/ssgo/discover"
	"github.com/ssgo/log"
	"github.com/ssgo/standard"
	"github.com/ssgo/u"
)

type WebServiceOptions struct {
	Priority int
	NoDoc    bool
	NoBody   bool
	NoLog200 bool
	Host     string
	Ext      Map
	Limiters []*Limiter
}

type webServiceType struct {
	authLevel        int
	method           string
	path             string
	pathMatcher      *regexp.Regexp
	pathArgs         []string
	parmsNum         int
	inType           reflect.Type
	inIndex          int
	headersType      reflect.Type
	headersIndex     int
	requestIndex     int
	httpRequestIndex int
	//uploaderIndex       int
	responseIndex       int
	responseWriterIndex int
	loggerIndex         int
	callerIndex         int
	funcType            reflect.Type
	funcValue           reflect.Value
	options             WebServiceOptions
	data                Map
	memo                string
}

var webServices = make(map[string]*webServiceType)
var regexWebServices = make([]*webServiceType, 0)
var webServicesLock = sync.RWMutex{}
var webServicesList = make([]*webServiceType, 0)

//var regexWebServicesLock = sync.RWMutex{}

var inFilters = make([]func(*map[string]any, *Request, *Response, *log.Logger) any, 0)
var outFilters = make([]func(map[string]any, *Request, *Response, any, *log.Logger) (any, bool), 0)
var errorHandle func(any, *Request, *Response) any
var webAuthChecker func(int, *log.Logger, *string, map[string]any, *Request, *Response, *WebServiceOptions) (pass bool, object any)
var webAuthCheckers = map[int]func(int, *log.Logger, *string, map[string]any, *Request, *Response, *WebServiceOptions) (pass bool, object any){}
var webAuthFailedData any
var verifyFailedData any
var limitFailedData any
var usedSessionIdKey string

// var usedClientIdKey string
var usedDeviceIdKey string
var usedClientAppKey string
var sessionIdMaker func() string

// var sessionCreator func() string
// var sessionObjects = map[*Request]map[reflect.Type]any{}
var injectObjects = map[reflect.Type]any{}
var injectFunctions = map[reflect.Type]func() any{}

func resetWebServiceMemory() {
	webServices = make(map[string]*webServiceType)
	regexWebServices = make([]*webServiceType, 0)
	//webServicesLock = sync.RWMutex{}
	webServicesList = make([]*webServiceType, 0)
	inFilters = make([]func(*map[string]any, *Request, *Response, *log.Logger) any, 0)
	outFilters = make([]func(map[string]any, *Request, *Response, any, *log.Logger) (any, bool), 0)
	errorHandle = nil
	webAuthChecker = nil
	webAuthCheckers = map[int]func(int, *log.Logger, *string, map[string]any, *Request, *Response, *WebServiceOptions) (pass bool, object any){}
	webAuthFailedData = nil
	verifyFailedData = nil
	limitFailedData = nil
	usedSessionIdKey = ""
	usedDeviceIdKey = ""
	usedClientAppKey = ""
	sessionIdMaker = nil
	injectObjects = map[reflect.Type]any{}
	injectFunctions = map[reflect.Type]func() any{}
}

// 设置 SessionKey，自动在 Header 中产生，AsyncStart 的客户端支持自动传递
//func SetSessionKey(inSessionKey string) {
//	if usedSessionIdKey == "" {
//		usedSessionIdKey = inSessionKey
//	}
//}

func SetClientKeys(deviceIdKey, clientAppKey, sessionIdKey string) {
	usedDeviceIdKey = deviceIdKey
	usedClientAppKey = clientAppKey
	usedSessionIdKey = sessionIdKey
}

func SetUserId(request *http.Request, userId string) {
	request.Header.Set(standard.DiscoverHeaderUserId, userId)
}
func (request *Request) SetUserId(userId string) {
	SetUserId(request.Request, userId)
}

func SetSessionId(request *http.Request, sessionId string) {
	request.Header.Set(usedSessionIdKey, sessionId)
	request.Header.Set(standard.DiscoverHeaderSessionId, sessionId)
}
func (request *Request) SetSessionId(sessionId string) {
	SetSessionId(request.Request, sessionId)
}

func SetSessionIdMaker(maker func() string) {
	sessionIdMaker = maker
}

//// 设置 Session ID 生成器
//func SetSessionCreator(creator func() string) {
//	sessionCreator = creator
//}
//
//// 获取 SessionKey
//func GetSessionKey() string {
//	return usedSessionIdKey
//}

// 获取 SessionId
func GetSessionId(request *http.Request) string {
	sessionId := request.Header.Get(usedSessionIdKey)
	if sessionId == "" && !Config.SessionWithoutCookie {
		if ck, err := request.Cookie(usedSessionIdKey); err == nil {
			sessionId = ck.Value
		}
	}
	if sessionId == "" {
		sessionId = request.Header.Get(standard.DiscoverHeaderSessionId)
	}
	return sessionId
}
func (request *Request) GetSessionId() string {
	return GetSessionId(request.Request)
}

//// 设置一个生命周期在 Request 中的对象，请求中可以使用对象类型注入参数方便调用
//func SetSessionInject(request *http.Request, obj any) {
//	if sessionObjects[request] == nil {
//		sessionObjects[request] = map[reflect.Type]any{}
//	}
//	sessionObjects[request][reflect.TypeOf(obj)] = obj
//}
//
//// 获取本生命周期中指定类型的 Session 对象
//func GetSessionInject(request *http.Request, dataType reflect.Type) any {
//	if sessionObjects[request] == nil {
//		return nil
//	}
//	return sessionObjects[request][dataType]
//}

// 设置一个注入对象，请求中可以使用对象类型注入参数方便调用
func SetInject(data any) {
	injectObjects[reflect.TypeOf(data)] = data
}
func SetInjectFunc(factory func() any) {
	injectFunctions[reflect.TypeOf(factory())] = factory
}

// 获取一个注入对象
func GetInject(dataType reflect.Type) any {
	if injectObjects[dataType] != nil {
		return injectObjects[dataType]
	} else if injectFunctions[dataType] != nil {
		return injectFunctions[dataType]()
	}
	return nil
}

type Context struct {
	Logger *log.Logger
}

func (ctx *Context) SetLogger(logger *log.Logger) {
	ctx.Logger = logger
}

type HostRegister struct {
	host string
}

func Host(host string) HostRegister {
	return HostRegister{host: host}
}

func (host *HostRegister) Static(path, rootPath string) {
	StaticByHost(path, rootPath, host.host)
}

//func (host *HostRegister) SetStaticGZFile(path string, data []byte) {
//	SetStaticGZFileByHost(path, data, host.host)
//}
//func (host *HostRegister) SetStaticFile(path string, data []byte) {
//	SetStaticFileByHost(path, data, host.host)
//}

func (host *HostRegister) Register(authLevel int, path string, serviceFunc any, memo string) {
	RestfulWithOptions(authLevel, "", path, serviceFunc, memo, WebServiceOptions{Host: host.host})
}
func (host *HostRegister) Restful(authLevel int, method, path string, serviceFunc any, memo string) {
	RestfulWithOptions(authLevel, method, path, serviceFunc, memo, WebServiceOptions{Host: host.host})
}
func (host *HostRegister) RegisterWithOptions(authLevel int, path string, serviceFunc any, memo string, options WebServiceOptions) {
	options.Host = host.host
	RestfulWithOptions(authLevel, "", path, serviceFunc, memo, options)
}
func (host *HostRegister) RestfulWithOptions(authLevel int, method, path string, serviceFunc any, memo string, options WebServiceOptions) {
	options.Host = host.host
	RestfulWithOptions(authLevel, method, path, serviceFunc, memo, options)
}
func (host *HostRegister) Unregister(method, path string) {
	unregister(method, path, WebServiceOptions{Host: host.host})
}
func (host *HostRegister) RegisterSimpleWebsocket(authLevel int, path string, onOpen any, memo string) {
	RegisterSimpleWebsocketWithOptions(authLevel, path, onOpen, memo, WebServiceOptions{Host: host.host})
}
func (host *HostRegister) RegisterSimpleWebsocketWithOptions(authLevel int, path string, onOpen any, memo string, options WebServiceOptions) {
	options.Host = host.host
	RegisterWebsocketWithOptions(authLevel, path, nil, onOpen, nil, nil, nil, true, memo, options)
}
func (host *HostRegister) RegisterWebsocket(authLevel int, path string, updater *websocket.Upgrader,
	onOpen any,
	onClose any,
	decoder func(data any) (action string, request map[string]any, err error),
	encoder func(action string, data any) any, memo string) *ActionRegister {
	return RegisterWebsocketWithOptions(authLevel, path, updater, onOpen, onClose, decoder, encoder, false, memo, WebServiceOptions{Host: host.host})
}
func (host *HostRegister) RegisterWebsocketWithOptions(authLevel int, path string, updater *websocket.Upgrader,
	onOpen any,
	onClose any,
	decoder func(data any) (action string, request map[string]any, err error),
	encoder func(action string, data any) any, isSimple bool, memo string, options WebServiceOptions) *ActionRegister {
	options.Host = host.host
	return RegisterWebsocketWithOptions(authLevel, path, updater, onOpen, onClose, decoder, encoder, false, memo, options)
}

// 注册服务
func Register(authLevel int, path string, serviceFunc any, memo string) {
	Restful(authLevel, "", path, serviceFunc, memo)
}

// 注册服务
func Restful(authLevel int, method, path string, serviceFunc any, memo string) {
	RestfulWithOptions(authLevel, method, path, serviceFunc, memo, WebServiceOptions{})
}

// 注册服务
func RegisterWithOptions(authLevel int, path string, serviceFunc any, memo string, options WebServiceOptions) {
	RestfulWithOptions(authLevel, "", path, serviceFunc, memo, options)
}

// 注册服务
func RestfulWithOptions(authLevel int, method, path string, serviceFunc any, memo string, options WebServiceOptions) {
	s, err := makeCachedService(serviceFunc)
	if err != nil {
		logError(err.Error(), "authLevel", authLevel, "priority", options.Priority, "path", path, "method", method)
		return
	}

	s.authLevel = authLevel
	s.options = options
	s.method = method
	s.path = path
	s.memo = memo
	finder, err := regexp.Compile("{(.*?)}")
	if err == nil {
		keyName := regexp.QuoteMeta(path)
		finds := finder.FindAllStringSubmatch(path, 20)
		for _, found := range finds {
			keyName = strings.Replace(keyName, regexp.QuoteMeta(found[0]), "(.*?)", 1)
			s.pathArgs = append(s.pathArgs, found[1])
		}
		if len(s.pathArgs) > 0 {
			s.pathMatcher, err = regexp.Compile("^" + keyName + "$")
			if err != nil {
				logError(err.Error(), Map{
					"authLevel": authLevel,
					"priority":  options.Priority,
					"path":      path,
					"method":    method,
				})
				//log.Print("Register	Compile	", err)
			}
			// 检查是否有已经存在的项
			deleteIndex := -1
			deleteIndex2 := -1
			webServicesLock.RLock()
			for i, s := range regexWebServices {
				if s.options.Host == options.Host && s.method == method && s.path == path {
					deleteIndex = i
					break
				}
			}
			for i, s := range webServicesList {
				if s.options.Host == options.Host && s.method == method && s.path == path {
					deleteIndex2 = i
					break
				}
			}
			webServicesLock.RUnlock()

			webServicesLock.Lock()
			regexWebServices = append(regexWebServices, s)
			if deleteIndex >= 0 {
				//regexWebServices = append(regexWebServices[0:deleteIndex], regexWebServices[deleteIndex+1:]...)
				newList := regexWebServices[0:deleteIndex]
				if len(regexWebServices) > deleteIndex {
					newList = append(newList, regexWebServices[deleteIndex+1:]...)
				}
				regexWebServices = newList
			}
			webServicesList = append(webServicesList, s)
			if deleteIndex2 >= 0 {
				//webServicesList = append(webServicesList[0:deleteIndex2], webServicesList[deleteIndex2+1:]...)
				newList := webServicesList[0:deleteIndex2]
				if len(webServicesList) > deleteIndex2 {
					newList = append(newList, webServicesList[deleteIndex2+1:]...)
				}
				webServicesList = newList
			}
			webServicesLock.Unlock()
		}
	}
	if s.pathMatcher == nil {
		// 检查是否有已经存在的项
		deleteIndex2 := -1
		webServicesLock.RLock()
		for i, s := range webServicesList {
			if s.options.Host == options.Host && s.method == method && s.path == path {
				deleteIndex2 = i
				break
			}
		}
		webServicesLock.RUnlock()

		webServicesLock.Lock()
		webServices[fmt.Sprint(options.Host, method, path)] = s
		webServicesList = append(webServicesList, s)
		if deleteIndex2 >= 0 {
			newList := webServicesList[0:deleteIndex2]
			if len(webServicesList) > deleteIndex2 {
				newList = append(newList, webServicesList[deleteIndex2+1:]...)
			}
			webServicesList = newList
		}
		webServicesLock.Unlock()
	}
}

// 注册服务
func Unregister(method, path string) {
	unregister(method, path, WebServiceOptions{})
}

// 注册服务
func unregister(method, path string, options WebServiceOptions) {
	webServicesLock.RLock()
	isRegexWebService := false
	if webServices[fmt.Sprint(options.Host, method, path)] == nil {
		isRegexWebService = true
	}
	webServicesLock.RUnlock()

	if !isRegexWebService {
		webServicesLock.Lock()
		delete(webServices, fmt.Sprint(options.Host, method, path))
		for i, s := range webServicesList {
			if s.options.Host == options.Host && s.method == method && s.path == path {
				webServicesList = append(webServicesList[0:i], webServicesList[i+1:]...)
				break
			}
		}
		webServicesLock.Unlock()
	} else {
		webServicesLock.Lock()
		for i, s := range regexWebServices {
			if s.options.Host == options.Host && s.method == method && s.path == path {
				regexWebServices = append(regexWebServices[0:i], regexWebServices[i+1:]...)
				break
			}
		}
		for i, s := range webServicesList {
			if s.options.Host == options.Host && s.method == method && s.path == path {
				webServicesList = append(webServicesList[0:i], webServicesList[i+1:]...)
				break
			}
		}
		webServicesLock.Unlock()
	}
}

// 设置前置过滤器
func SetInFilter(filter func(in *map[string]any, request *Request, response *Response, logger *log.Logger) (out any)) {
	inFilters = append(inFilters, filter)
}

// 设置后置过滤器
func SetOutFilter(filter func(in map[string]any, request *Request, response *Response, out any, logger *log.Logger) (newOut any, isOver bool)) {
	outFilters = append(outFilters, filter)
}

func SetAuthChecker(authChecker func(authLevel int, logger *log.Logger, url *string, in map[string]any, request *Request, response *Response, options *WebServiceOptions) (pass bool, object any)) {
	webAuthChecker = authChecker
}

func AddAuthChecker(authLevels []int, authChecker func(authLevel int, logger *log.Logger, url *string, in map[string]any, request *Request, response *Response, options *WebServiceOptions) (pass bool, object any)) {
	for _, al := range authLevels {
		webAuthCheckers[al] = authChecker
	}
}

func SetAuthFailedData(data any) {
	webAuthFailedData = data
}

func SetVerifyFailedData(data any) {
	verifyFailedData = data
}

func SetLimitFailedData(data any) {
	limitFailedData = data
}

func SetErrorHandle(myErrorHandle func(err any, request *Request, response *Response) any) {
	errorHandle = myErrorHandle
}

//func StringValue(v reflect.Value) reflect.Value {
//	v = Elem(v)
//	k := v.Kind()
//	switch k {
//	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
//		return reflect.ValueOf(strconv.FormatInt(v.Int(), 10))
//	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
//		return reflect.ValueOf(strconv.FormatUint(v.Uint(), 10))
//	case reflect.Float32, reflect.Float64:
//		return reflect.ValueOf(strconv.FormatFloat(v.Float(), 'f', -1, 64))
//	case reflect.Bool:
//		if v.Bool() {
//			return reflect.ValueOf(true)
//		} else {
//			return reflect.ValueOf(true)
//		}
//	case reflect.String:
//		return v
//	default:
//		if (k == reflect.Slice || k == reflect.Array) && v.Type().Kind() == reflect.Uint8 {
//			var buf []uint8
//			if k == reflect.Array {
//				buf = make([]uint8, v.Len(), v.Len())
//				for i := range buf {
//					buf[i] = v.Index(i).Interface().(uint8)
//				}
//			} else {
//				buf = v.Interface().([]uint8)
//			}
//			return reflect.ValueOf(string(buf))
//		} else {
//			return reflect.ValueOf(fmt.Sprint(v.Interface()))
//		}
//	}
//}
//
//func IntValue(v reflect.Value) reflect.Value {
//	return reflect.ValueOf(int(intValue(v).Int()))
//}
//
//func Int8Value(v reflect.Value) reflect.Value {
//	return reflect.ValueOf(int8(intValue(v).Int()))
//}
//
//func Int16Value(v reflect.Value) reflect.Value {
//	return reflect.ValueOf(int16(intValue(v).Int()))
//}
//
//func Int32Value(v reflect.Value) reflect.Value {
//	return reflect.ValueOf(int32(intValue(v).Int()))
//}
//
//func Int64Value(v reflect.Value) reflect.Value {
//	return reflect.ValueOf(int64(intValue(v).Int()))
//}
//
//func intValue(v reflect.Value) reflect.Value {
//	v = Elem(v)
//	k := v.Kind()
//	switch k {
//	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
//		return v
//	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
//		return reflect.ValueOf(int64(v.Uint()))
//	case reflect.Float32, reflect.Float64:
//		return reflect.ValueOf(strconv.FormatFloat(v.Float(), 'f', -1, 64))
//	case reflect.Bool:
//		if v.Bool() {
//			return reflect.ValueOf(true)
//		} else {
//			return reflect.ValueOf(true)
//		}
//	case reflect.String:
//		return v
//	default:
//		if (k == reflect.Slice || k == reflect.Array) && v.Type().Kind() == reflect.Uint8 {
//			var buf []uint8
//			if k == reflect.Array {
//				buf = make([]uint8, v.Len(), v.Len())
//				for i := range buf {
//					buf[i] = v.Index(i).Interface().(uint8)
//				}
//			} else {
//				buf = v.Interface().([]uint8)
//			}
//			return reflect.ValueOf(string(buf))
//		} else {
//			return reflect.ValueOf(fmt.Sprint(v.Interface()))
//		}
//	}
//}

func doWebService(service *webServiceType, request *Request, response *Response, args map[string]any,
	result any, requestLogger *log.Logger, object any) (webResult any) {
	// 反射调用
	if result != nil {
		return result
	}

	request.Set("registerTag", fmt.Sprint(service.options.Host, service.method, service.path))

	// 限制访问频率
	if service.options.Limiters != nil && len(service.options.Limiters) > 0 {
		for _, l := range service.options.Limiters {
			if ok, value := l.Check(args, request.Request, requestLogger); !ok {
				response.WriteHeader(429)
				if limitFailedData != nil {
					dataStr := u.Json(limitFailedData)
					if strings.Contains(dataStr, "{{") {
						dataStr = strings.ReplaceAll(dataStr, "{{LIMITED_FROM}}", l.fromKey)
						dataStr = strings.ReplaceAll(dataStr, "{{LIMITED_VALUE}}", value)
						webResult = u.UnJsonMap(dataStr)
					} else {
						webResult = limitFailedData
					}
				} else {
					webResult = "too many requests"
				}
				return
			}
		}
	}

	// 生成参数
	var parms = make([]reflect.Value, service.parmsNum)
	if service.inIndex >= 0 {
		if service.inType.Kind() == reflect.Map && service.inType.Elem().Kind() == reflect.Interface {
			parms[service.inIndex] = reflect.ValueOf(args)
		} else {
			in := reflect.New(service.inType).Interface()
			u.Convert(args, in)
			// 验证参数有效性
			if service.inType.Kind() == reflect.Struct {
				if ok, field := VerifyStruct(in, requestLogger); !ok {
					response.WriteHeader(400)
					if verifyFailedData != nil {
						dataStr := u.Json(verifyFailedData)
						if strings.Contains(dataStr, "{{") {
							dataStr = strings.ReplaceAll(dataStr, "{{FAILED_FIELDS}}", field)
							webResult = u.UnJsonMap(dataStr)
						} else {
							webResult = verifyFailedData
						}
					} else {
						webResult = field + " verify failed"
					}
					return
				}
			}
			parms[service.inIndex] = reflect.ValueOf(in).Elem()
		}
	}
	if service.headersIndex >= 0 {
		//parms[service.headersIndex] = reflect.ValueOf(&request.Header)
		headersMap := map[string]string{}
		for k, v := range request.Header {
			if len(v) == 1 {
				headersMap[strings.ReplaceAll(k, "-", "")] = v[0]
			} else if len(v) > 1 {
				headersMap[strings.ReplaceAll(k, "-", "")] = strings.Join(v, ", ")
			}
		}
		if service.headersType.Kind() == reflect.Map && service.headersType.Elem().Kind() == reflect.String {
			parms[service.headersIndex] = reflect.ValueOf(headersMap)
		} else {
			headers := reflect.New(service.headersType).Interface()
			u.Convert(headersMap, headers)
			// 验证参数有效性
			if service.headersType.Kind() == reflect.Struct {
				if ok, field := VerifyStruct(headers, requestLogger); !ok {
					response.WriteHeader(400)
					if verifyFailedData != nil {
						dataStr := u.Json(verifyFailedData)
						if strings.Contains(dataStr, "{{") {
							dataStr = strings.ReplaceAll(dataStr, "{{FAILED_FIELDS}}", field)
							webResult = u.UnJsonMap(dataStr)
						} else {
							webResult = verifyFailedData
						}
					} else {
						webResult = field + " verify failed"
					}
					return
				}
			}
			parms[service.headersIndex] = reflect.ValueOf(headers).Elem()
		}
	}
	if service.requestIndex >= 0 {
		parms[service.requestIndex] = reflect.ValueOf(request)
	}
	if service.httpRequestIndex >= 0 {
		parms[service.httpRequestIndex] = reflect.ValueOf(request.Request)
	}
	//if service.uploaderIndex >= 0 {
	//	parms[service.uploaderIndex] = reflect.ValueOf(&Uploader{request: request.Request})
	//}
	if service.responseIndex >= 0 {
		parms[service.responseIndex] = reflect.ValueOf(response)
	}
	if service.responseWriterIndex >= 0 {
		parms[service.responseWriterIndex] = reflect.ValueOf(response.Writer)
	}
	if service.loggerIndex >= 0 {
		parms[service.loggerIndex] = reflect.ValueOf(requestLogger)
	}
	if service.callerIndex >= 0 {
		caller := &discover.Caller{Request: request.Request}
		parms[service.callerIndex] = reflect.ValueOf(caller)
	}
	for i, parm := range parms {
		if parm.Kind() == reflect.Invalid {
			st := service.funcType.In(i)
			isset := false
			if st.Kind() == reflect.Struct || (st.Kind() == reflect.Ptr && st.Elem().Kind() == reflect.Struct) {
				if object != nil && reflect.TypeOf(object) == st {
					parms[i] = reflect.ValueOf(object)
					isset = true
				} else {
					injectObj := GetInject(st)
					if injectObj != nil {
						parms[i] = getInjectObjectValueWithLogger(injectObj, requestLogger)
						isset = true
					}
				}
			}
			if isset == false {
				parms[i] = reflect.New(st).Elem()
			}
		}
	}
	outs := service.funcValue.Call(parms)
	if len(outs) > 0 {
		webResult = outs[0].Interface()
	} else {
		webResult = ""
	}
	return
}

func getInjectObjectValueWithLogger(injectObj any, requestLogger *log.Logger) reflect.Value {
	injectObjValue := reflect.ValueOf(injectObj)
	if setLoggerMethod, found := injectObjValue.Type().MethodByName("CopyByLogger"); found && setLoggerMethod.Type.NumIn() == 2 && setLoggerMethod.Type.NumOut() == 1 && setLoggerMethod.Type.In(1).String() == "*log.Logger" && setLoggerMethod.Type.Out(0).String() == injectObjValue.Type().String() {
		outs := setLoggerMethod.Func.Call([]reflect.Value{injectObjValue, reflect.ValueOf(requestLogger)})
		injectObjValue = outs[0]
	} else if setLoggerMethod, found := injectObjValue.Type().MethodByName("SetLogger"); found && setLoggerMethod.Type.NumIn() == 2 && setLoggerMethod.Type.In(1).String() == "*log.Logger" {
		newInjectObj := injectObjValue.Elem().Interface()
		injectObjValue = reflect.New(injectObjValue.Type().Elem())
		injectObjValue.Elem().Set(reflect.ValueOf(newInjectObj))
		setLoggerMethod.Func.Call([]reflect.Value{injectObjValue, reflect.ValueOf(requestLogger)})
	}
	return injectObjValue
}

//func makePrintable(data []byte) {
//	n := len(data)
//	for i := 0; i < n; i++ {
//		c := data[i]
//		if c == '\t' || c == '\n' || c == '\r' {
//			data[i] = ' '
//			//} else if c < 32 || c > 126 {
//			//} else if c < 32 {
//			//	data[i] = '?'
//		}
//	}
//}

func makeCachedService(matchedServie any) (*webServiceType, error) {
	// 类型或参数返回值个数不对
	funcType := reflect.TypeOf(matchedServie)
	if funcType.Kind() != reflect.Func {
		return nil, fmt.Errorf("bad Service")
	}

	// 参数类型不对
	targetService := new(webServiceType)
	targetService.parmsNum = funcType.NumIn()
	targetService.inIndex = -1
	targetService.headersIndex = -1
	targetService.requestIndex = -1
	targetService.httpRequestIndex = -1
	//targetService.uploaderIndex = -1
	targetService.responseIndex = -1
	targetService.responseWriterIndex = -1
	targetService.loggerIndex = -1
	targetService.callerIndex = -1
	for i := 0; i < targetService.parmsNum; i++ {
		t := funcType.In(i)
		if t.String() == "*s.Request" {
			targetService.requestIndex = i
		} else if t.String() == "*http.Request" {
			targetService.httpRequestIndex = i
			//} else if t.String() == "*s.Uploader" {
			//	targetService.uploaderIndex = i
		} else if t.String() == "*s.Response" {
			targetService.responseIndex = i
		} else if t.String() == "http.ResponseWriter" {
			targetService.responseWriterIndex = i
		} else if t.String() == "*log.Logger" {
			targetService.loggerIndex = i
			//} else if t.String() == "*http.Header" {
			//	targetService.headersIndex = i
		} else if t.String() == "*discover.Caller" {
			targetService.callerIndex = i
		} else if t.Kind() == reflect.Struct || (t.Kind() == reflect.Map && t.Elem().Kind() == reflect.Interface) || (t.Kind() == reflect.Map && t.Elem().Kind() == reflect.String) {
			if targetService.inType == nil {
				targetService.inIndex = i
				targetService.inType = t
			} else if targetService.headersType == nil {
				targetService.headersIndex = i
				targetService.headersType = t
			}
		}
	}

	targetService.funcType = funcType
	targetService.funcValue = reflect.ValueOf(matchedServie)
	return targetService, nil
}

func makeBytesResult(data any) []byte {
	excludeKeys := u.MakeExcludeUpperKeys(data, "")
	bytesResult, err := json.Marshal(data)
	if err != nil || (len(bytesResult) == 4 && string(bytesResult) == "null") {
		t := reflect.TypeOf(data)
		if t.Kind() == reflect.Slice {
			bytesResult = []byte("[]")
		}
		if t.Kind() == reflect.Map {
			bytesResult = []byte("{}")
		}
	}
	if !Config.KeepKeyCase {
		u.FixUpperCase(bytesResult, excludeKeys)
	}
	return bytesResult
}
