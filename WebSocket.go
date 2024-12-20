package s

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ssgo/discover"
	"github.com/ssgo/log"

	"github.com/gorilla/websocket"
	"github.com/ssgo/u"
)

type websocketServiceType struct {
	authLevel             int
	path                  string
	pathMatcher           *regexp.Regexp
	pathArgs              []string
	updater               *websocket.Upgrader
	openParmsNum          int
	openInType            reflect.Type
	openInIndex           int
	openRequestIndex      int
	openHttpRequestIndex  int
	openLoggerIndex       int
	openClientIndex       int
	openCallerIndex       int
	openHeadersType       reflect.Type
	openHeadersIndex      int
	openFuncType          reflect.Type
	openFuncValue         reflect.Value
	sessionType           reflect.Type
	closeParmsNum         int
	closeClientIndex      int
	closeRequestIndex     int
	closeHttpRequestIndex int
	closeLoggerIndex      int
	closeSessionIndex     int
	closeFuncType         reflect.Type
	closeFuncValue        reflect.Value
	decoder               func(any) (string, map[string]any, error)
	encoder               func(string, any) any
	actions               map[string]*websocketActionType
	isSimple              bool
	options               WebServiceOptions
	memo                  string
}

type websocketActionType struct {
	authLevel    int
	priority     int
	parmsNum     int
	inType       reflect.Type
	inIndex      int
	clientIndex  int
	bytesIndex   int
	sessionIndex int
	loggerIndex  int
	funcType     reflect.Type
	funcValue    reflect.Value
	memo         string
}
type ActionRegister struct {
	websocketName        string
	websocketServiceType *websocketServiceType
}

var websocketServices = make(map[string]*websocketServiceType)
var regexWebsocketServices = make([]*websocketServiceType, 0)
var websocketServicesLock = sync.RWMutex{}
var websocketServicesList = make([]*websocketServiceType, 0)

//var regexWebsocketServicesLock = sync.RWMutex{}

var webSocketActionAuthChecker func(int, *string, *string, map[string]any, *Request, any) bool

func resetWebSocketMemory() {
	websocketServices = make(map[string]*websocketServiceType)
	regexWebsocketServices = make([]*websocketServiceType, 0)
	websocketServicesList = make([]*websocketServiceType, 0)
	webSocketActionAuthChecker = nil
}

// 注册Websocket服务
func RegisterSimpleWebsocket(authLevel int, path string, onOpen any, memo string) {
	RegisterSimpleWebsocketWithOptions(authLevel, path, onOpen, memo, WebServiceOptions{})
}

func RegisterSimpleWebsocketWithOptions(authLevel int, path string, onOpen any, memo string, options WebServiceOptions) {
	RegisterWebsocketWithOptions(authLevel, path, nil, onOpen, nil, nil, nil, true, memo, options)
}

func RegisterWebsocket(authLevel int, path string, updater *websocket.Upgrader,
	onOpen any,
	onClose any,
	decoder func(data any) (action string, request map[string]any, err error),
	encoder func(action string, data any) any, memo string) *ActionRegister {
	return RegisterWebsocketWithOptions(authLevel, path, updater, onOpen, onClose, decoder, encoder, false, memo, WebServiceOptions{})
}

// 注册Websocket服务
func RegisterWebsocketWithOptions(authLevel int, path string, updater *websocket.Upgrader,
	onOpen any,
	onClose any,
	decoder func(data any) (action string, request map[string]any, err error),
	encoder func(action string, data any) any, isSimple bool, memo string, options WebServiceOptions) *ActionRegister {

	s := new(websocketServiceType)
	s.isSimple = isSimple
	s.authLevel = authLevel
	s.options = options
	s.path = path
	s.memo = memo
	if updater == nil {
		s.updater = new(websocket.Upgrader)
	} else {
		s.updater = updater
	}
	s.decoder = decoder
	s.encoder = encoder
	s.actions = make(map[string]*websocketActionType)

	s.openFuncType = reflect.TypeOf(onOpen)
	if s.openFuncType != nil {
		s.openParmsNum = s.openFuncType.NumIn()
		s.openInIndex = -1
		s.openHeadersIndex = -1
		s.openClientIndex = -1
		s.openCallerIndex = -1
		s.openRequestIndex = -1
		s.openHttpRequestIndex = -1
		s.openLoggerIndex = -1
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
			} else if t.String() == "*s.Request" {
				s.openRequestIndex = i
			} else if t.String() == "*http.Request" {
				s.openHttpRequestIndex = i
			} else if t.String() == "*log.Logger" {
				s.openLoggerIndex = i
				//} else if t.String() == "*http.Header" {
				//	s.openHeadersIndex = i
			} else if t.String() == "*websocket.Conn" {
				s.openClientIndex = i
			} else if t.String() == "*discover.Caller" {
				s.openCallerIndex = i
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
		s.closeRequestIndex = -1
		s.closeHttpRequestIndex = -1
		s.closeLoggerIndex = -1
		s.closeSessionIndex = -1
		s.closeFuncValue = reflect.ValueOf(onClose)
		for i := 0; i < s.closeParmsNum; i++ {
			t := s.closeFuncType.In(i)
			if t == s.sessionType {
				s.closeSessionIndex = i
				s.sessionType = t
			} else if t.String() == "*websocket.Conn" {
				s.closeClientIndex = i
			} else if t.String() == "*s.Request" {
				s.closeRequestIndex = i
			} else if t.String() == "*http.Request" {
				s.closeHttpRequestIndex = i
			} else if t.String() == "*log.Logger" {
				s.closeLoggerIndex = i
			}
		}
	}

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
				})
				//log.Print("RegisterWebsocket	Compile	", err)
			}
			//regexWebsocketServices[path] = s
			regexWebsocketServices = append(regexWebsocketServices, s)
			websocketServicesList = append(websocketServicesList, s)
		}
	}
	if s.pathMatcher == nil {
		websocketServicesLock.Lock()
		websocketServices[fmt.Sprint(options.Host, path)] = s
		websocketServicesLock.Unlock()
		websocketServicesList = append(websocketServicesList, s)
	}

	return &ActionRegister{websocketName: path, websocketServiceType: s}
}

func (ar *ActionRegister) RegisterAction(authLevel int, actionName string, action any, memo string) {
	ar.RegisterActionWithPriority(authLevel, 0, actionName, action, memo)
}
func (ar *ActionRegister) RegisterActionWithPriority(authLevel, priority int, actionName string, action any, memo string) {
	a := new(websocketActionType)
	a.authLevel = authLevel
	a.priority = priority
	a.memo = memo
	a.funcType = reflect.TypeOf(action)
	if a.funcType != nil {
		a.parmsNum = a.funcType.NumIn()
		a.inIndex = -1
		a.sessionIndex = -1
		a.loggerIndex = -1
		a.clientIndex = -1
		a.funcValue = reflect.ValueOf(action)
		for i := 0; i < a.parmsNum; i++ {
			t := a.funcType.In(i)
			if t == ar.websocketServiceType.sessionType {
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
			} else if t.String() == "*log.Logger" {
				a.loggerIndex = i
			}
		}
	}
	ar.websocketServiceType.actions[actionName] = a
}

func SetActionAuthChecker(authChecker func(authLevel int, url *string, action *string, in map[string]any, request *Request, sess any) bool) {
	webSocketActionAuthChecker = authChecker
}

func doWebsocketService(ws *websocketServiceType, request *Request, response *Response, authLevel int, args map[string]any, startTime *time.Time, requestLogger *log.Logger, sessionObject any) {
	//byteArgs, _ := json.Marshal(args)
	//byteHeaders, _ := json.Marshal(headers)

	message := "OK"
	client, err := ws.updater.Upgrade(response.Writer, request.Request, nil)
	if err != nil {
		message = err.Error()
		response.WriteHeader(500)
	}

	writeLog(requestLogger, "WSOPEN", nil, 0, request.Request, response, args, startTime, authLevel, Map{
		"message": message,
	})

	if err == nil {
		var sessionValue reflect.Value
		if ws.openFuncType != nil {
			var openParms = make([]reflect.Value, ws.openParmsNum)
			if ws.openInIndex >= 0 {
				in := reflect.New(ws.openInType).Interface()
				u.Convert(args, in)
				openParms[ws.openInIndex] = reflect.ValueOf(in).Elem()
			}
			if ws.openHeadersIndex >= 0 {
				//openParms[ws.openRequestIndex] = reflect.ValueOf(&request.Header)
				headersParm := reflect.New(ws.openHeadersType).Interface()
				u.Convert(getLogHeaders(request.Request), headersParm)
				openParms[ws.openHeadersIndex] = reflect.ValueOf(headersParm).Elem()
			}
			if ws.openRequestIndex >= 0 {
				openParms[ws.openRequestIndex] = reflect.ValueOf(request)
			}
			if ws.openHttpRequestIndex >= 0 {
				openParms[ws.openHttpRequestIndex] = reflect.ValueOf(request.Request)
			}
			if ws.openLoggerIndex >= 0 {
				openParms[ws.openLoggerIndex] = reflect.ValueOf(requestLogger)
			}
			if ws.openClientIndex >= 0 {
				openParms[ws.openClientIndex] = reflect.ValueOf(client)
			}
			if ws.openCallerIndex >= 0 {
				caller := &discover.Caller{Request: request.Request}
				openParms[ws.openCallerIndex] = reflect.ValueOf(caller)
			}

			for i, parm := range openParms {
				if parm.Kind() == reflect.Invalid {
					st := ws.openFuncType.In(i)
					isset := false
					if st.Kind() == reflect.Struct || (st.Kind() == reflect.Ptr && st.Elem().Kind() == reflect.Struct) {
						injectObj := GetInject(st)
						if injectObj != nil {
							openParms[i] = getInjectObjectValueWithLogger(injectObj, requestLogger)
							isset = true
						}
					}
					if !isset {
						openParms[i] = reflect.New(st).Elem()
					}
				}
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
		}

		if !ws.isSimple {
			for {
				msg := new(any)
				err := client.ReadJSON(msg)
				if err != nil {
					break
				}

				var actionName string
				var messageData map[string]any
				if ws.decoder != nil {
					actionName, messageData, err = ws.decoder(*msg)
					if err != nil {
						requestLogger.Error(err.Error(), Map{
							"message": u.String(*msg)[0:1024],
							"ip":      getRealIp(request.Request),
							"method":  request.Method,
							"host":    request.Host,
							"uri":     request.RequestURI,
						})
						//log.Printf("ERROR	Read a bad message	%s	%s	%s", getRealIp(request), request.RequestURI, fmt.Sprint(*msg))
					}
				} else {
					actionName = ""
					mapMsg, isMap := (*msg).(map[string]any)
					if isMap {
						messageData = mapMsg
						if messageData["action"] != "" {
							actionName = u.String(messageData["action"])
						}
					} else {
						messageData = map[string]any{"data": *msg}
					}
				}
				// 异步调用 action 处理
				action := ws.actions[actionName]
				if action == nil {
					action = ws.actions[""]
				}
				if action == nil {
					continue
				}

				//printableMsg, _ := json.Marshal(messageData)
				if webSocketActionAuthChecker != nil {
					if !webSocketActionAuthChecker(action.authLevel, &request.RequestURI, &actionName, messageData, request, sessionValue) {
						logInMsg := makeLogableData(requestLogger, reflect.ValueOf(messageData), noLogOutputFields, Config.LogOutputArrayNum, Config.LogOutputFieldSize, 1).Interface()
						writeLog(requestLogger, "WSREJECT", nil, 0, request.Request, response, args, startTime, authLevel, Map{
							"inAction":  actionName,
							"inMessage": logInMsg,
						})
						response.WriteHeader(403)
						continue
					}
				}

				actionStartTime := time.Now()
				outAction, outData, outLen, err := doWebsocketAction(ws, actionName, action, client, request, messageData, sessionValue, requestLogger, sessionObject)
				if err == nil {
					logInMsg := makeLogableData(requestLogger, reflect.ValueOf(messageData), noLogOutputFields, Config.LogOutputArrayNum, Config.LogOutputFieldSize, 1).Interface()
					logOutMsg := makeLogableData(requestLogger, reflect.ValueOf(outData), noLogOutputFields, Config.LogOutputArrayNum, Config.LogOutputFieldSize, 1).Interface()
					if Config.LogWebsocketAction {
						writeLog(requestLogger, "WSACTION", nil, outLen, request.Request, response, args, &actionStartTime, authLevel, Map{
							"inAction":   actionName,
							"inMessage":  logInMsg,
							"outAction":  outAction,
							"outMessage": logOutMsg,
						})
					}
					//log.Printf("WSACTION	%s	%s	%s	%.6f	%s", getRealIp(request), request.RequestURI, actionName, usedTime, string(printableMsg))
				} else {
					logInMsg := makeLogableData(requestLogger, reflect.ValueOf(messageData), noLogOutputFields, Config.LogOutputArrayNum, Config.LogOutputFieldSize, 1).Interface()
					logOutMsg := makeLogableData(requestLogger, reflect.ValueOf(outData), noLogOutputFields, Config.LogOutputArrayNum, Config.LogOutputFieldSize, 1).Interface()
					writeLog(requestLogger, "WSACTIONERROR", nil, outLen, request.Request, response, args, &actionStartTime, authLevel, Map{
						"inAction":   actionName,
						"inMessage":  logInMsg,
						"outAction":  outAction,
						"outMessage": logOutMsg,
						"error":      err.Error(),
					})
					//log.Printf("WSERROR	%s	%s	%s	%.6f	%s	%s", getRealIp(request), request.RequestURI, actionName, usedTime, string(printableMsg), err.Error())
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
				if ws.closeRequestIndex >= 0 {
					closeParms[ws.closeRequestIndex] = reflect.ValueOf(request)
				}
				if ws.closeHttpRequestIndex >= 0 {
					closeParms[ws.closeHttpRequestIndex] = reflect.ValueOf(request.Request)
				}
				if ws.closeLoggerIndex >= 0 {
					closeParms[ws.closeLoggerIndex] = reflect.ValueOf(requestLogger)
				}

				for i, parm := range closeParms {
					if parm.Kind() == reflect.Invalid {
						st := ws.openFuncType.In(i)
						isset := false
						if st.Kind() == reflect.Struct || (st.Kind() == reflect.Ptr && st.Elem().Kind() == reflect.Struct) {
							injectObj := GetInject(st)
							if injectObj != nil {
								closeParms[i] = getInjectObjectValueWithLogger(injectObj, requestLogger)
								isset = true
							}
						}
						if !isset {
							closeParms[i] = reflect.New(st).Elem()
						}
					}
				}

				ws.closeFuncValue.Call(closeParms)
			}
			_ = client.Close()
			writeLog(requestLogger, "WSCLOSE", nil, 0, request.Request, response, args, startTime, authLevel, nil)
		}
	}
}

func doWebsocketAction(ws *websocketServiceType, actionName string, action *websocketActionType, client *websocket.Conn, request *Request, data map[string]any, sess reflect.Value, requestLogger *log.Logger, sessionObject any) (string, any, int, error) {
	var messageParms = make([]reflect.Value, action.parmsNum)
	if action.inType != nil {
		in := reflect.New(action.inType).Interface()
		u.Convert(data, in)
		messageParms[action.inIndex] = reflect.ValueOf(in).Elem()
	}
	if action.sessionIndex >= 0 {
		messageParms[action.sessionIndex] = sess
	}
	if action.clientIndex >= 0 {
		messageParms[action.clientIndex] = reflect.ValueOf(client)
	}
	if action.loggerIndex >= 0 {
		messageParms[action.loggerIndex] = reflect.ValueOf(requestLogger)
	}
	for i, parm := range messageParms {
		if parm.Kind() == reflect.Invalid {
			st := action.funcType.In(i)
			isset := false
			if st.Kind() == reflect.Struct || (st.Kind() == reflect.Ptr && st.Elem().Kind() == reflect.Struct) {
				if sessionObject != nil && reflect.TypeOf(sessionObject) == st {
					messageParms[i] = reflect.ValueOf(sessionObject)
					isset = true
				} else {
					injectObj := GetInject(st)
					if injectObj != nil {
						messageParms[i] = getInjectObjectValueWithLogger(injectObj, requestLogger)
						isset = true
					}
				}
			}
			if !isset {
				messageParms[i] = reflect.New(st).Elem()
			}
		}
	}

	outs := action.funcValue.Call(messageParms)
	var outAction string
	var outData any
	var outLen int
	if len(outs) > 0 {
		outAction = actionName
		if len(outs) > 1 {
			outAction = outs[0].String()
			outData = outs[1].Interface()
		} else {
			outData = outs[0].Interface()
		}

		var outBytes []byte
		var err error
		if ws.encoder != nil {
			outBytes, err = json.Marshal(ws.encoder(outAction, outData))
		} else {
			outDataType := reflect.TypeOf(outData)
			var outDataMap map[string]any
			if outDataType.Kind() == reflect.Map && outDataType.Elem().Kind() == reflect.Interface {
				outDataMap = outData.(map[string]any)
			} else if outDataType.Kind() == reflect.Struct {
				u.Convert(outData, &outDataMap)
			} else {
				outDataMap = map[string]any{}
				outDataMap["data"] = outData
			}
			outDataMap["action"] = outAction
			outBytes, err = json.Marshal(outDataMap)
		}
		outLen = len(outBytes)

		if err != nil {
			return outAction, outData, outLen, err
		}
		if !Config.KeepKeyCase {
			u.FixUpperCase(outBytes, nil)
		}
		err = client.WriteMessage(websocket.TextMessage, outBytes)
		if err != nil {
			return outAction, outData, outLen, err
		}
	}

	return outAction, outData, outLen, nil
}
