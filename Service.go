package service

import (
	"github.com/ssgo/base"
	"net/http"
	"strings"
	"io/ioutil"
	"encoding/json"
	"log"
	"time"
	"github.com/mitchellh/mapstructure"
	"reflect"
	"regexp"
	"github.com/gorilla/websocket"
	"fmt"
)

type webServiceType struct {
	pathMatcher   *regexp.Regexp
	pathArgs      []string
	isService     bool
	parmsNum      int
	inType        reflect.Type
	inIndex       int
	headersType   reflect.Type
	headersIndex  int
	requestIndex  int
	responseIndex int
	funcType      reflect.Type
	funcValue     reflect.Value
}

type websocketServiceType struct {
	pathMatcher       *regexp.Regexp
	pathArgs          []string
	updater           *websocket.Upgrader
	openParmsNum      int
	openInType        reflect.Type
	openInIndex       int
	openRequestIndex  int
	openClientIndex   int
	openHeadersType   reflect.Type
	openHeadersIndex  int
	openFuncType      reflect.Type
	openFuncValue     reflect.Value
	sessionType       reflect.Type
	closeParmsNum     int
	closeClientIndex  int
	closeSessionIndex int
	closeFuncType     reflect.Type
	closeFuncValue    reflect.Value
	decoder func(*interface{}) (string, map[string]interface{}, error)
	actions map[string]*websocketActionType
}

type websocketActionType struct {
	parmsNum     int
	inType       reflect.Type
	inIndex      int
	clientIndex  int
	bytesIndex   int
	sessionIndex int
	funcType     reflect.Type
	funcValue    reflect.Value
}

//type websocketService struct {
//	Updater   *websocket.Upgrader
//	OnOpen    interface{}
//	OnMessage interface{}
//	OnClose   interface{}
//}

var webServices = make(map[string]*webServiceType)
var regexWebServices = make(map[string]*webServiceType)

var websocketServices = make(map[string]*websocketServiceType)
var regexWebsocketServices = make(map[string]*websocketServiceType)

var inFilters = make([]func(map[string]interface{}) *Result, 0)
var outFilters = make([]func(map[string]interface{}, *Result) *Result, 0)

//var contexts = make(map[string]interface{})
//var cachedWebsocketActions = make(map[string]map[string]*websocketActionType)
var recordLogs = true

var config = struct {
	Listen string
}{}

func init() {
	base.LoadConfig("service", &config)
	if config.Listen == "" {
		config.Listen = ":8001"
	}
}

// 注册服务
func Register(name string, service interface{}) {
	s, err := makeCachedService(service)
	if err != nil {
		log.Fatalln("bad service", name, service)
		return
	}

	finder, err := regexp.Compile("\\{(.+?)\\}")
	if err == nil {
		keyName := regexp.QuoteMeta(name)
		finds := finder.FindAllStringSubmatch(name, 20)
		for _, found := range finds {
			keyName = strings.Replace(keyName, regexp.QuoteMeta(found[0]), "(.+?)", 1)
			s.pathArgs = append(s.pathArgs, found[1])
		}
		if len(s.pathArgs) > 0 {
			s.pathMatcher, _ = regexp.Compile("^" + keyName + "$")
			regexWebServices[name] = s
		}
	}
	if s.pathMatcher == nil {
		webServices[name] = s
	}
}

//// 注册以正则匹配的服务
//func RegisterByRegex(name string, service interface{}) {
//	s, err := makeCachedService(service)
//	if err != nil {
//		log.Fatalln("bad service", name, service)
//		return
//	}
//	regexWebServices[name] = s
//}

// 注册Websocket服务
func RegisterWebsocket(name string, updater *websocket.Upgrader,
	onOpen interface{},
//onMessage func(*websocket.Conn, []byte),
	onClose interface{},
	decoder func(*interface{}) (string, map[string]interface{}, error)) {

	s := new(websocketServiceType)
	if updater == nil {
		s.updater = new(websocket.Upgrader)
	} else {
		s.updater = updater
	}
	s.decoder = decoder
	s.actions = make(map[string]*websocketActionType)

	s.openFuncType = reflect.TypeOf(onOpen)
	if s.openFuncType != nil {
		s.openParmsNum = s.openFuncType.NumIn()
		s.openInIndex = -1
		s.openHeadersIndex = -1
		//s.openPathArgsIndex = -1
		s.openClientIndex = -1
		s.openRequestIndex = -1
		s.openFuncValue = reflect.ValueOf(onOpen)
		for i := 0; i < s.openParmsNum; i++ {
			t := s.openFuncType.In(i)
			if t.Kind() == reflect.Struct {
				if s.openInType == nil {
					s.openInIndex = i
					s.openInType = t
				} else if s.openHeadersType == nil {
					s.openHeadersIndex = i
					s.openHeadersType = t
				}
			} else if t.String() == "*http.Request" {
				s.openRequestIndex = i
			} else if t.String() == "*websocket.Conn" {
				s.openClientIndex = i
			}
		}

		if s.openFuncType.NumOut() > 0 {
			s.sessionType = s.openFuncType.Out(0)
		}
	}

	s.closeFuncType = reflect.TypeOf(onClose)
	if s.closeFuncType != nil {
		s.closeParmsNum = s.closeFuncType.NumIn()
		s.closeClientIndex = -1
		s.closeSessionIndex = -1
		s.closeFuncValue = reflect.ValueOf(onClose)
		for i := 0; i < s.closeParmsNum; i++ {
			t := s.closeFuncType.In(i)
			if t == s.sessionType {
				s.closeSessionIndex = i
				s.sessionType = t
			} else if t.String() == "*websocket.Conn" {
				s.closeClientIndex = i
			}
		}
	}

	finder, err := regexp.Compile("\\{(.+?)\\}")
	if err == nil {
		keyName := regexp.QuoteMeta(name)
		finds := finder.FindAllStringSubmatch(name, 20)
		for _, found := range finds {
			keyName = strings.Replace(keyName, regexp.QuoteMeta(found[0]), "(.+?)", 1)
			s.pathArgs = append(s.pathArgs, found[1])
		}
		if len(s.pathArgs) > 0 {
			s.pathMatcher, _ = regexp.Compile("^" + keyName + "$")
			regexWebsocketServices[name] = s
		}
	}
	if s.pathMatcher == nil {
		websocketServices[name] = s
	}
}

func RegisterWebsocketAction(serviceName, actionName string, action interface{}) {
	s := websocketServices[serviceName]
	if s == nil {
		s = regexWebsocketServices[serviceName]
	}
	if s == nil {
		log.Fatalln("no websocket servive", serviceName, "for register action")
		return
	}

	a := new(websocketActionType)
	a.funcType = reflect.TypeOf(action)
	if a.funcType != nil {
		a.parmsNum = a.funcType.NumIn()
		a.inIndex = -1
		a.clientIndex = -1
		a.funcValue = reflect.ValueOf(action)
		for i := 0; i < a.parmsNum; i++ {
			t := a.funcType.In(i)
			if t == s.sessionType {
				a.sessionIndex = i
			} else if t.Kind() == reflect.Struct {
				if a.inType == nil {
					a.inIndex = i
					a.inType = t
				}
			} else if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
				a.bytesIndex = i
			} else if t.String() == "*websocket.Conn" {
				a.clientIndex = i
			}
		}
	}
	s.actions[actionName] = a
}

//// 设置上下文内容，可以在服务函数的参数中直接得到并使用
//func SetContext(name string, context interface{}) {
//	contexts[name] = context
//}

// 设置前置过滤器
func SetInFilter(filter func(map[string]interface{}) *Result) {
	inFilters = append(inFilters, filter)
}

// 设置后置过滤器
func SetOutFilter(filter func(map[string]interface{}, *Result) *Result) {
	outFilters = append(outFilters, filter)
}

// 启动服务
func Start() {
	http.Handle("/", &routeHandler{})
	err := http.ListenAndServe(config.Listen, nil)
	if err != nil {
		log.Fatalln("Failed to start service", err)
	}
}

func EnableLogs(enabled bool) {
	recordLogs = enabled
}

func ResetAllSets() {
	webServices = make(map[string]*webServiceType)
	regexWebServices = make(map[string]*webServiceType)
	inFilters = make([]func(map[string]interface{}) *Result, 0)
	outFilters = make([]func(map[string]interface{}, *Result) *Result, 0)
	websocketServices = make(map[string]*websocketServiceType)
	regexWebsocketServices = make(map[string]*websocketServiceType)
	recordLogs = true
}

type routeHandler struct{}

func (*routeHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	startTime := time.Now()

	// 获取路径
	requestPath := request.RequestURI
	pos := strings.LastIndex(requestPath, "?")
	if pos != -1 {
		requestPath = requestPath[0:pos]
	}
	args := make(map[string]interface{})

	// 先看缓存中是否有
	s := webServices[requestPath]
	var ws *websocketServiceType = nil
	if s == nil {
		ws = websocketServices[requestPath]
	}

	// 未匹配到缓存，尝试匹配新的Service
	if s == nil && ws == nil {
		//for k, v := range regexWebServices {
		//	matched, err := regexp.MatchString(k, requestPath)
		//	if err == nil && matched {
		//		s = v
		//		break
		//	}
		//}
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

	// Headers，未来可以优化日志记录，最近访问过的头部信息可省略
	headers := make(map[string]interface{})
	for k, v := range request.Header {
		headerKey := strings.Replace(k, "-", "", -1)
		if len(v) > 1 {
			headers[headerKey] = v
		} else {
			headers[headerKey] = v[0]
		}
	}

	// 处理 Websocket
	if ws != nil {
		doWebsocketService(ws, request, response, &args, &headers, &startTime)
	} else if s != nil {
		doWebService(s, request, response, &args, &headers, &startTime)
	}
}

func doWebsocketService(ws *websocketServiceType, request *http.Request, response http.ResponseWriter, args *map[string]interface{}, headers *map[string]interface{}, startTime *time.Time) {
	// 前置过滤器
	var result *Result = nil
	for _, filter := range inFilters {
		result = filter(*args)
		if result != nil {
			break
		}
	}
	byteArgs, _ := json.Marshal(*args)
	byteHeaders, _ := json.Marshal(*headers)

	code := 200
	message := "OK"
	client, err := ws.updater.Upgrade(response, request, nil)
	if err != nil {
		code = 500
		message = err.Error()
		response.WriteHeader(500)
	}

	if recordLogs {
		nowTime := time.Now()
		usedTime := float32(nowTime.Nanosecond()-startTime.Nanosecond()) / 1e6
		*startTime = nowTime
		log.Printf("WSOPEN\t%s\t%s\t%.6f\t%d\t%s\t%s\t%s\n", request.RemoteAddr, request.RequestURI, usedTime, code, message, byteArgs, byteHeaders)
	}

	if err == nil {
		var sessionValue reflect.Value
		if ws.openFuncType != nil {
			var openParms = make([]reflect.Value, ws.openParmsNum)
			if ws.openInType != nil {
				in := reflect.New(ws.openInType).Interface()
				mapstructure.WeakDecode(*args, in)
				openParms[ws.openInIndex] = reflect.ValueOf(in).Elem()
			}
			if ws.openHeadersType != nil {
				inHeaders := reflect.New(ws.openHeadersType).Interface()
				mapstructure.WeakDecode(*headers, inHeaders)
				openParms[ws.openHeadersIndex] = reflect.ValueOf(inHeaders).Elem()
			}
			if ws.openRequestIndex >= 0 {
				openParms[ws.openRequestIndex] = reflect.ValueOf(request)
			}
			if ws.openClientIndex >= 0 {
				openParms[ws.openClientIndex] = reflect.ValueOf(client)
			}

			//client.SetCloseHandler(func(closeCode int, closeMessage string) error {
			//	log.Println(" >>>>", code, message)
			//	code = closeCode
			//	message = closeMessage
			//	log.Println(" >>>> Close", code, message)
			//	return nil
			//})

			outs := ws.openFuncValue.Call(openParms)
			if len(outs) > 0 {
				sessionValue = outs[0]
			}

			for {
				msg := new(interface{})
				err := client.ReadJSON(msg)
				if err != nil {
					break
				}

				if ws.decoder != nil {
					actionName, messageData, err := ws.decoder(msg)
					if err != nil {
						log.Fatalln("Read a bad message", request.RemoteAddr, request.RequestURI, msg)
					}

					// 异步调用 action 处理
					action := ws.actions[actionName]
					if action != nil {
						doWebsocketAction(action, client, &messageData, sessionValue)
					} else if ws.actions[""] != nil {
						doWebsocketAction(ws.actions[""], client, &messageData, sessionValue)
					}
				}
			}

			// 调用 onClose
			if ws.closeFuncType != nil {
				var closeParms = make([]reflect.Value, ws.closeParmsNum)
				if ws.closeSessionIndex >= 0 {
					closeParms[ws.closeSessionIndex] = sessionValue
				}
				if ws.closeClientIndex >= 0 {
					closeParms[ws.closeClientIndex] = reflect.ValueOf(client)
				}
				ws.closeFuncValue.Call(closeParms)
			}

			if recordLogs {
				usedTime := float32(time.Now().Nanosecond()-startTime.Nanosecond()) / 1e6
				log.Printf("WSCLOSE\t%s\t%s\t%.6f\t%d\t%s\t%s\t%s\n", request.RemoteAddr, request.RequestURI, usedTime, code, message, byteArgs, byteHeaders)
			}

		}
	}
}

func doWebsocketAction(action *websocketActionType, client *websocket.Conn, data *map[string]interface{}, sess reflect.Value) {
	//startt := time.Now()
	var messageParms = make([]reflect.Value, action.parmsNum)
	if action.inType != nil {
		in := reflect.New(action.inType).Interface()
		mapstructure.WeakDecode(*data, in)
		messageParms[action.inIndex] = reflect.ValueOf(in).Elem()
	}
	if action.sessionIndex >= 0 {
		messageParms[action.sessionIndex] = sess
	}
	if action.clientIndex >= 0 {
		messageParms[action.clientIndex] = reflect.ValueOf(client)
	}
	action.funcValue.Call(messageParms)
	//stopt := time.Now()
	//log.Println(" !!!@@@###$$$%%%^^^&&&***", stopt.Nanosecond() - startt.Nanosecond())
}

func doWebService(service *webServiceType, request *http.Request, response http.ResponseWriter, args *map[string]interface{}, headers *map[string]interface{}, startTime *time.Time) {
	// 前置过滤器
	var result *Result = nil
	for _, filter := range inFilters {
		result = filter(*args)
		if result != nil {
			break
		}
	}

	// 反射调用
	if result == nil {
		// 生成参数
		var parms = make([]reflect.Value, service.parmsNum)
		if service.inType != nil {
			in := reflect.New(service.inType).Interface()
			mapstructure.WeakDecode(*args, in)
			parms[service.inIndex] = reflect.ValueOf(in).Elem()
		}
		if service.headersType != nil {
			inHeaders := reflect.New(service.headersType).Interface()
			mapstructure.WeakDecode(*headers, inHeaders)
			parms[service.headersIndex] = reflect.ValueOf(inHeaders).Elem()
		}
		if service.requestIndex >= 0 {
			parms[service.requestIndex] = reflect.ValueOf(request)
		}
		if service.responseIndex >= 0 {
			parms[service.responseIndex] = reflect.ValueOf(response)
		}

		outs := service.funcValue.Call(parms)
		if service.isService {
			code := int(outs[0].Int())
			message := outs[1].String()
			data := outs[2].Interface()
			result = &Result{code, message, data}
		} else {
			data := outs[0].Interface()
			result = &Result{200, "OK", data}
		}
	}

	// 后置过滤器
	if len(outFilters) > 0 {
		byteResults := makeBytesResult(result.Code, result.Message, result.Data)
		mapedResult := make(map[string]interface{})
		err := json.Unmarshal(byteResults, &mapedResult)
		if err == nil {
			result.Data = mapedResult["data"]
			for _, filter := range outFilters {
				newResult := filter(*args, result)
				if newResult != nil {
					// 使用新的结果
					result = newResult
					break
				}
			}
		}
	}

	// 记录访问日志
	if result != nil {
		if service.isService {
			response.Write(makeBytesResult(result.Code, result.Message, result.Data))
		} else {
			var outBytes []byte
			if reflect.TypeOf(result.Data).Kind() == reflect.String {
				outBytes = []byte(result.Data.(string))
			} else {
				outBytes = result.Data.([]byte)
			}
			response.Write(outBytes)
		}
		if recordLogs {
			usedTime := float32(time.Now().Nanosecond()-startTime.Nanosecond()) / 1e6
			byteArgs, _ := json.Marshal(*args)
			byteHeaders, _ := json.Marshal(*headers)
			log.Printf("ACCESS\t%s\t%s\t%.6f\t%d\t%s\t%s\n\t%s", request.RemoteAddr, request.RequestURI, usedTime, result.Code, result.Message, byteArgs, byteHeaders)
		}
	}
}

func makeCachedService(matchedServie interface{}) (*webServiceType, error) {
	// 类型或参数返回值个数不对
	funcType := reflect.TypeOf(matchedServie)
	if funcType.Kind() != reflect.Func || (funcType.NumOut() != 3 && funcType.NumOut() != 1) {
		return nil, fmt.Errorf("Bad Service")
	}

	// 参数类型不对
	targetService := new(webServiceType)
	targetService.parmsNum = funcType.NumIn()
	targetService.inIndex = -1
	targetService.headersIndex = -1
	targetService.requestIndex = -1
	targetService.responseIndex = -1
	for i := 0; i < targetService.parmsNum; i++ {
		t := funcType.In(i)

		if t.Kind() == reflect.Struct {
			if targetService.inType == nil {
				targetService.inIndex = i
				targetService.inType = t
			} else if targetService.headersType == nil {
				targetService.headersIndex = i
				targetService.headersType = t
			}
		} else if t.String() == "*http.Request" {
			targetService.requestIndex = i
		} else if t.String() == "http.ResponseWriter" {
			targetService.responseIndex = i
		}
	}

	// 返回值类型不对
	if funcType.NumOut() == 1 {
		targetService.isService = false
		outType := funcType.Out(0)
		if outType.Kind() != reflect.String && (outType.Kind() != reflect.Slice || outType.Elem().Kind() != reflect.Uint8) {
			return nil, fmt.Errorf("Bad Service Outputs")
		}
	} else {
		targetService.isService = true
		outCodeType := funcType.Out(0)
		outMessageType := funcType.Out(1)
		if outCodeType.Kind() != reflect.Int || outMessageType.Kind() != reflect.String {
			return nil, fmt.Errorf("Bad Service Outputs")
		}
	}

	targetService.funcType = funcType
	targetService.funcValue = reflect.ValueOf(matchedServie)
	return targetService, nil
}

//func makeCachedWebsocketService(matchedServie *websocketService) (*websocketServiceType, error) {
//	targetService := new(websocketServiceType)
//
//	// 类型或参数返回值个数不对
//	targetService.openFuncType = reflect.TypeOf(matchedServie.OnOpen)
//	targetService.messageFuncType = reflect.TypeOf(matchedServie.OnMessage)
//	targetService.closeFuncType = reflect.TypeOf(matchedServie.OnClose)
//
//	// open 回调处理
//	if targetService.openFuncType != nil {
//		targetService.openParmsNum = targetService.openFuncType.NumIn()
//		targetService.openInIndex = -1
//		targetService.openHeadersIndex = -1
//		targetService.openPathArgsIndex = -1
//		targetService.openClientIndex = -1
//		targetService.openRequestIndex = -1
//		for i := 0; i < targetService.openParmsNum; i++ {
//			t := targetService.openFuncType.In(i)
//			if t.Kind() == reflect.Struct || t.Kind() == reflect.Map {
//				if targetService.openInType == nil {
//					targetService.openInIndex = i
//					targetService.openInType = t
//				} else if targetService.openHeadersType == nil {
//					targetService.openHeadersIndex = i
//					targetService.openHeadersType = t
//				}
//			} else if t.String() == "*http.Request" {
//				targetService.openRequestIndex = i
//			} else if t.String() == "*websocket.Conn" {
//				targetService.openClientIndex = i
//			}
//		}
//	}
//
//	if targetService.messageFuncType != nil {
//		targetService.messageParmsNum = targetService.messageFuncType.NumIn()
//		targetService.messageInIndex = -1
//		targetService.messageClientIndex = -1
//		for i := 0; i < targetService.messageParmsNum; i++ {
//			t := targetService.messageFuncType.In(i)
//			if t.Kind() == reflect.Struct || t.Kind() == reflect.Map {
//				if targetService.messageInType == nil {
//					targetService.messageInIndex = i
//					targetService.messageInType = t
//				}
//			} else if t.String() == "*websocket.Conn" {
//				targetService.messageClientIndex = i
//			}
//		}
//	}
//
//	return targetService, nil
//}

type Result struct {
	Code    int
	Message string
	Data    interface{}
}

func makeBytesResult(code int, message string, data interface{}) []byte {
	bytesResult, err := json.Marshal(map[string]interface{}{
		"code":    code,
		"message": message,
		"data":    data,
	})
	if err != nil {
		bytesResult = []byte("{\"code\":511,\"message\":\"Bad Result\",\"data\":null}")
	}
	n := len(bytesResult)
	inObject := false
	for i := 1; i < n-1; i++ {
		if bytesResult[i] == '{' {
			inObject = true
		} else if bytesResult[i] == '}' {
			inObject = false
		}
		if bytesResult[i] == '"' && (bytesResult[i-1] == '{' || (bytesResult[i-1] == ',' && inObject)) && (bytesResult[i+1] >= 'A' && bytesResult[i+1] <= 'Z') {
			bytesResult[i+1] += 32
		}
	}

	return bytesResult
}
