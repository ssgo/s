package s

import (
	"encoding/json"
	"fmt"
	"github.com/ssgo/log"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ssgo/u"
)

type websocketServiceType struct {
	authLevel         int
	priority          int
	host              string
	path              string
	pathMatcher       *regexp.Regexp
	pathArgs          []string
	updater           *websocket.Upgrader
	openParmsNum      int
	openInType        reflect.Type
	openInIndex       int
	openRequestIndex  int
	openLoggerIndex   int
	openClientIndex   int
	openHeadersType   reflect.Type
	openHeadersIndex  int
	openFuncType      reflect.Type
	openFuncValue     reflect.Value
	sessionType       reflect.Type
	closeParmsNum     int
	closeClientIndex  int
	closeRequestIndex int
	closeLoggerIndex  int
	closeSessionIndex int
	closeFuncType     reflect.Type
	closeFuncValue    reflect.Value
	decoder           func(interface{}) (string, map[string]interface{}, error)
	encoder           func(string, interface{}) interface{}
	actions           map[string]*websocketActionType
	isSimple          bool
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
}
type ActionRegister struct {
	websocketName        string
	websocketServiceType *websocketServiceType
}

var websocketServices = make(map[string]*websocketServiceType)
var regexWebsocketServices = make([]*websocketServiceType, 0)

var webSocketActionAuthChecker func(int, *string, *string, map[string]interface{}, *http.Request, interface{}) bool

// 注册Websocket服务
func RegisterSimpleWebsocket(authLevel int, path string, onOpen interface{}) {
	RegisterSimpleWebsocketWithPriority(authLevel, 0, "", path, onOpen)
}

func RegisterSimpleWebsocketWithPriority(authLevel int, priority int, host, path string, onOpen interface{}) {
	RegisterWebsocketWithPriority(authLevel, priority, host, path, nil, onOpen, nil, nil, nil, true)
}

func RegisterWebsocket(authLevel int, path string, updater *websocket.Upgrader,
	onOpen interface{},
	onClose interface{},
	decoder func(data interface{}) (action string, request map[string]interface{}, err error),
	encoder func(action string, data interface{}) interface{}) *ActionRegister {
	return RegisterWebsocketWithPriority(authLevel, 0, "", path, updater, onOpen, onClose, decoder, encoder, false)
}

// 注册Websocket服务
func RegisterWebsocketWithPriority(authLevel, priority int, host, path string, updater *websocket.Upgrader,
	onOpen interface{},
	onClose interface{},
	decoder func(data interface{}) (action string, request map[string]interface{}, err error),
	encoder func(action string, data interface{}) interface{}, isSimple bool) *ActionRegister {

	s := new(websocketServiceType)
	s.isSimple = isSimple
	s.authLevel = authLevel
	s.priority = priority
	s.host = host
	s.path = path
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
		s.openRequestIndex = -1
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
			} else if t.String() == "*http.Request" {
				s.openRequestIndex = i
			} else if t.String() == "*log.Logger" {
				s.openLoggerIndex = i
				//} else if t.String() == "*http.Header" {
				//	s.openHeadersIndex = i
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
		s.closeRequestIndex = -1
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
			} else if t.String() == "*http.Request" {
				s.closeRequestIndex = i
			} else if t.String() == "*log.Logger" {
				s.closeRequestIndex = i
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
					"priority":  priority,
					"path":      path,
				})
				//log.Print("RegisterWebsocket	Compile	", err)
			}
			//regexWebsocketServices[path] = s
			regexWebsocketServices = append(regexWebsocketServices, s)
		}
	}
	if s.pathMatcher == nil {
		websocketServices[fmt.Sprint(host, path)] = s
	}

	return &ActionRegister{websocketName: path, websocketServiceType: s}
}

func (ar *ActionRegister) RegisterAction(authLevel int, actionName string, action interface{}) {
	ar.RegisterActionWithPriority(authLevel, 0, actionName, action)
}
func (ar *ActionRegister) RegisterActionWithPriority(authLevel, priority int, actionName string, action interface{}) {
	a := new(websocketActionType)
	a.authLevel = authLevel
	a.priority = priority
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

func SetActionAuthChecker(authChecker func(authLevel int, url *string, action *string, in map[string]interface{}, request *http.Request, sess interface{}) bool) {
	webSocketActionAuthChecker = authChecker
}

func doWebsocketService(ws *websocketServiceType, request *http.Request, response *Response, authLevel int, args map[string]interface{}, headers map[string]string, startTime *time.Time, requestLogger *log.Logger, sessionObject interface{}) {
	//byteArgs, _ := json.Marshal(args)
	//byteHeaders, _ := json.Marshal(headers)

	message := "OK"
	client, err := ws.updater.Upgrade(response.writer, request, nil)
	if err != nil {
		message = err.Error()
		response.WriteHeader(500)
	}

	writeLog(requestLogger, "WSOPEN", nil, 0, request, response, args, headers, startTime, authLevel, Map{
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
				u.Convert(headers, headersParm)
				openParms[ws.openHeadersIndex] = reflect.ValueOf(headersParm).Elem()
			}
			if ws.openRequestIndex >= 0 {
				openParms[ws.openRequestIndex] = reflect.ValueOf(request)
			}
			if ws.openLoggerIndex >= 0 {
				openParms[ws.openLoggerIndex] = reflect.ValueOf(requestLogger)
			}
			if ws.openClientIndex >= 0 {
				openParms[ws.openClientIndex] = reflect.ValueOf(client)
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
					if isset == false {
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
				msg := new(interface{})
				err := client.ReadJSON(msg)
				if err != nil {
					break
				}

				var actionName string
				var messageData map[string]interface{}
				if ws.decoder != nil {
					actionName, messageData, err = ws.decoder(*msg)
					if err != nil {
						requestLogger.Error(err.Error(), Map{
							"message": u.String(*msg)[0:1024],
							"ip":      getRealIp(request),
							"method":  request.Method,
							"host":    request.Host,
							"uri":     request.RequestURI,
						})
						//log.Printf("ERROR	Read a bad message	%s	%s	%s", getRealIp(request), request.RequestURI, fmt.Sprint(*msg))
					}
				} else {
					actionName = ""
					mapMsg, isMap := (*msg).(map[string]interface{})
					if isMap {
						messageData = mapMsg
						if messageData["action"] != "" {
							actionName = u.String(messageData["action"])
						}
					} else {
						messageData = map[string]interface{}{"data": *msg}
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
					if webSocketActionAuthChecker(action.authLevel, &request.RequestURI, &actionName, messageData, request, sessionValue) == false {
						logInMsg := makeLogableData(reflect.ValueOf(messageData), logOutputFields, Config.LogOutputArrayNum, 1).Interface()
						writeLog(requestLogger, "WSREJECT", nil, 0, request, response, args, headers, startTime, authLevel, Map{
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
					logInMsg := makeLogableData(reflect.ValueOf(messageData), logOutputFields, Config.LogOutputArrayNum, 1).Interface()
					logOutMsg := makeLogableData(reflect.ValueOf(outData), logOutputFields, Config.LogOutputArrayNum, 1).Interface()
					if Config.LogWebsocketAction {
						writeLog(requestLogger, "WSACTION", nil, outLen, request, response, args, headers, &actionStartTime, authLevel, Map{
							"inAction":   actionName,
							"inMessage":  logInMsg,
							"outAction":  outAction,
							"outMessage": logOutMsg,
						})
					}
					//log.Printf("WSACTION	%s	%s	%s	%.6f	%s", getRealIp(request), request.RequestURI, actionName, usedTime, string(printableMsg))
				} else {
					logInMsg := makeLogableData(reflect.ValueOf(messageData), logOutputFields, Config.LogOutputArrayNum, 1).Interface()
					logOutMsg := makeLogableData(reflect.ValueOf(outData), logOutputFields, Config.LogOutputArrayNum, 1).Interface()
					writeLog(requestLogger, "WSACTIONERROR", nil, outLen, request, response, args, headers, &actionStartTime, authLevel, Map{
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
						if isset == false {
							closeParms[i] = reflect.New(st).Elem()
						}
					}
				}

				ws.closeFuncValue.Call(closeParms)
			}
		}

		_ = client.Close()
		writeLog(requestLogger, "WSCLOSE", nil, 0, request, response, args, headers, startTime, authLevel, nil)
	}
}

func doWebsocketAction(ws *websocketServiceType, actionName string, action *websocketActionType, client *websocket.Conn, request *http.Request, data map[string]interface{}, sess reflect.Value, requestLogger *log.Logger, sessionObject interface{}) (string, interface{}, int, error) {
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
			if isset == false {
				messageParms[i] = reflect.New(st).Elem()
			}
		}
	}

	outs := action.funcValue.Call(messageParms)
	var outAction string
	var outData interface{}
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
			var outDataMap map[string]interface{}
			if outDataType.Kind() == reflect.Map && outDataType.Elem().Kind() == reflect.Interface {
				outDataMap = outData.(map[string]interface{})
			} else if outDataType.Kind() == reflect.Struct {
				u.Convert(outData, &outDataMap)
			} else {
				outDataMap = map[string]interface{}{}
				outDataMap["data"] = outData
			}
			outDataMap["action"] = outAction
			outBytes, err = json.Marshal(outDataMap)
		}
		outLen = len(outBytes)

		if err != nil {
			return outAction, outData, outLen, err
		}
		u.FixUpperCase(outBytes, nil)
		err = client.WriteMessage(websocket.TextMessage, outBytes)
		if err != nil {
			return outAction, outData, outLen, err
		}
	}

	return outAction, outData, outLen, nil
}
