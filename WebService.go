package s

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/ssgo/discover"
	"github.com/ssgo/log"
	"github.com/ssgo/standard"
	"github.com/ssgo/u"
	"net/http"
	"reflect"
	"regexp"
	"strings"
)

type WebServiceOptions struct {
	Priority int
	NoBody   bool
	NoLog200 bool
}

type webServiceType struct {
	authLevel           int
	method              string
	host                string
	path                string
	pathMatcher         *regexp.Regexp
	pathArgs            []string
	parmsNum            int
	inType              reflect.Type
	inIndex             int
	headersType         reflect.Type
	headersIndex        int
	requestIndex        int
	uploaderIndex       int
	responseIndex       int
	responseWriterIndex int
	loggerIndex         int
	callerIndex         int
	funcType            reflect.Type
	funcValue           reflect.Value
	options             WebServiceOptions
}

var webServices = make(map[string]*webServiceType)
var regexWebServices = make([]*webServiceType, 0)

var inFilters = make([]func(map[string]interface{}, *http.Request, *Response) interface{}, 0)
var outFilters = make([]func(map[string]interface{}, *http.Request, *Response, interface{}) (interface{}, bool), 0)
var errorHandle func(interface{}, *http.Request, *Response) interface{}
var webAuthChecker func(int, *log.Logger, *string, map[string]interface{}, *http.Request, *Response) (pass bool, sessionObject interface{})
var webAuthFailedData interface{}
var usedSessionIdKey string

//var usedClientIdKey string
var usedDeviceIdKey string
var usedClientAppKey string

//var sessionCreator func() string
//var sessionObjects = map[*http.Request]map[reflect.Type]interface{}{}
var injectObjects = map[reflect.Type]interface{}{}
var injectFunctions = map[reflect.Type]func() interface{}{}

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

func SetSessionId(request *http.Request, sessionId string) {
	request.Header.Set(usedSessionIdKey, sessionId)
	request.Header.Set(standard.DiscoverHeaderSessionId, sessionId)
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
	if sessionId == "" {
		sessionId = request.Header.Get(standard.DiscoverHeaderSessionId)
	}
	return sessionId
}

//// 设置一个生命周期在 Request 中的对象，请求中可以使用对象类型注入参数方便调用
//func SetSessionInject(request *http.Request, obj interface{}) {
//	if sessionObjects[request] == nil {
//		sessionObjects[request] = map[reflect.Type]interface{}{}
//	}
//	sessionObjects[request][reflect.TypeOf(obj)] = obj
//}
//
//// 获取本生命周期中指定类型的 Session 对象
//func GetSessionInject(request *http.Request, dataType reflect.Type) interface{} {
//	if sessionObjects[request] == nil {
//		return nil
//	}
//	return sessionObjects[request][dataType]
//}

// 设置一个注入对象，请求中可以使用对象类型注入参数方便调用
func SetInject(data interface{}) {
	injectObjects[reflect.TypeOf(data)] = data
}
func SetInjectFunc(factory func() interface{}) {
	injectFunctions[reflect.TypeOf(factory())] = factory
}

// 获取一个注入对象
func GetInject(dataType reflect.Type) interface{} {
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

func (host *HostRegister) Register(authLevel int, path string, serviceFunc interface{}) {
	RestfulWithOptions(authLevel, "", host.host, path, serviceFunc, WebServiceOptions{})
}
func (host *HostRegister) Restful(authLevel int, method, path string, serviceFunc interface{}) {
	RestfulWithOptions(authLevel, method, host.host, path, serviceFunc, WebServiceOptions{})
}
func (host *HostRegister) RegisterWithOptions(authLevel int, path string, serviceFunc interface{}, options WebServiceOptions) {
	RestfulWithOptions(authLevel, "", host.host, path, serviceFunc, WebServiceOptions{})
}
func (host *HostRegister) RestfulWithOptions(authLevel int, method, path string, serviceFunc interface{}, options WebServiceOptions) {
	RestfulWithOptions(authLevel, method, host.host, path, serviceFunc, WebServiceOptions{})
}
func (host *HostRegister) RegisterSimpleWebsocket(authLevel int, path string, onOpen interface{}) {
	RegisterSimpleWebsocketWithOptions(authLevel, "", path, onOpen, WebServiceOptions{})
}

func (host *HostRegister) RegisterSimpleWebsocketWithOptions(authLevel int, path string, onOpen interface{}, options WebServiceOptions) {
	RegisterWebsocketWithOptions(authLevel, host.host, path, nil, onOpen, nil, nil, nil, true, options)
}
func (host *HostRegister) RegisterWebsocket(authLevel int, path string, updater *websocket.Upgrader,
	onOpen interface{},
	onClose interface{},
	decoder func(data interface{}) (action string, request map[string]interface{}, err error),
	encoder func(action string, data interface{}) interface{}) *ActionRegister {
	return RegisterWebsocketWithOptions(authLevel, host.host, path, updater, onOpen, onClose, decoder, encoder, false, WebServiceOptions{})
}
func (host *HostRegister) RegisterWebsocketWithOptions(authLevel int, path string, updater *websocket.Upgrader,
	onOpen interface{},
	onClose interface{},
	decoder func(data interface{}) (action string, request map[string]interface{}, err error),
	encoder func(action string, data interface{}) interface{}, isSimple bool, options WebServiceOptions) *ActionRegister {
	return RegisterWebsocketWithOptions(authLevel, host.host, path, updater, onOpen, onClose, decoder, encoder, false, options)
}

// 注册服务
func Register(authLevel int, path string, serviceFunc interface{}) {
	Restful(authLevel, "", path, serviceFunc)
}

// 注册服务
func Restful(authLevel int, method, path string, serviceFunc interface{}) {
	RestfulWithOptions(authLevel, method, "", path, serviceFunc, WebServiceOptions{})
}

// 注册服务
func RegisterWithOptions(authLevel int, host, path string, serviceFunc interface{}, options WebServiceOptions) {
	RestfulWithOptions(authLevel, "", host, path, serviceFunc, options)
}

// 注册服务
func RestfulWithOptions(authLevel int, method, host, path string, serviceFunc interface{}, options WebServiceOptions) {
	s, err := makeCachedService(serviceFunc)
	if err != nil {
		logError(err.Error(), "authLevel", authLevel, "priority", options.Priority, "path", path, "method", method)
		return
	}

	s.authLevel = authLevel
	s.options = options
	s.method = method
	s.host = host
	s.path = path
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
			regexWebServices = append(regexWebServices, s)
		}
	}
	if s.pathMatcher == nil {
		webServices[fmt.Sprint(host, method, path)] = s
	}
}

// 设置前置过滤器
func SetInFilter(filter func(in map[string]interface{}, request *http.Request, response *Response) (out interface{})) {
	inFilters = append(inFilters, filter)
}

// 设置后置过滤器
func SetOutFilter(filter func(in map[string]interface{}, request *http.Request, response *Response, out interface{}) (newOut interface{}, isOver bool)) {
	outFilters = append(outFilters, filter)
}

func SetAuthChecker(authChecker func(authLevel int, logger *log.Logger, url *string, in map[string]interface{}, request *http.Request, response *Response) (pass bool, sessionObject interface{})) {
	webAuthChecker = authChecker
}

func SetAuthFailedData(data interface{}) {
	webAuthFailedData = data
}

func SetErrorHandle(myErrorHandle func(err interface{}, request *http.Request, response *Response) interface{}) {
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

func doWebService(service *webServiceType, request *http.Request, response *Response, args map[string]interface{},
	result interface{}, requestLogger *log.Logger, sessionObject interface{}) (webResult interface{}) {
	// 反射调用
	if result != nil {
		return result
	}
	// 生成参数
	var parms = make([]reflect.Value, service.parmsNum)
	if service.inIndex >= 0 {
		if service.inType.Kind() == reflect.Map && service.inType.Elem().Kind() == reflect.Interface {
			parms[service.inIndex] = reflect.ValueOf(args)
		} else {
			in := reflect.New(service.inType).Interface()
			u.Convert(args, in)
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
			parms[service.headersIndex] = reflect.ValueOf(headers).Elem()
		}
	}
	if service.requestIndex >= 0 {
		parms[service.requestIndex] = reflect.ValueOf(request)
	}
	if service.uploaderIndex >= 0 {
		parms[service.uploaderIndex] = reflect.ValueOf(&Uploader{request: request})
	}
	if service.responseIndex >= 0 {
		parms[service.responseIndex] = reflect.ValueOf(response)
	}
	if service.responseWriterIndex >= 0 {
		parms[service.responseWriterIndex] = reflect.ValueOf(response.writer)
	}
	if service.loggerIndex >= 0 {
		parms[service.loggerIndex] = reflect.ValueOf(requestLogger)
	}
	if service.callerIndex >= 0 {
		caller := &discover.Caller{Request: request}
		parms[service.callerIndex] = reflect.ValueOf(caller)
	}
	for i, parm := range parms {
		if parm.Kind() == reflect.Invalid {
			st := service.funcType.In(i)
			isset := false
			if st.Kind() == reflect.Struct || (st.Kind() == reflect.Ptr && st.Elem().Kind() == reflect.Struct) {
				if sessionObject != nil && reflect.TypeOf(sessionObject) == st {
					parms[i] = reflect.ValueOf(sessionObject)
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

func getInjectObjectValueWithLogger(injectObj interface{}, requestLogger *log.Logger) reflect.Value {
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

func makeCachedService(matchedServie interface{}) (*webServiceType, error) {
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
	targetService.uploaderIndex = -1
	targetService.responseIndex = -1
	targetService.responseWriterIndex = -1
	targetService.loggerIndex = -1
	targetService.callerIndex = -1
	for i := 0; i < targetService.parmsNum; i++ {
		t := funcType.In(i)
		if t.String() == "*http.Request" {
			targetService.requestIndex = i
		} else if t.String() == "*s.Uploader" {
			targetService.uploaderIndex = i
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

func makeBytesResult(data interface{}) []byte {
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
	u.FixUpperCase(bytesResult, nil)
	return bytesResult
}
