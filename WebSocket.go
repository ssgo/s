package s

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
	"github.com/ssgo/base"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"
)

type websocketServiceType struct {
	authLevel         uint
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
	decoder           func(interface{}) (string, *map[string]interface{}, error)
	encoder           func(string, interface{}) interface{}
	actions           map[string]*websocketActionType
}

type websocketActionType struct {
	authLevel    uint
	parmsNum     int
	inType       reflect.Type
	inIndex      int
	clientIndex  int
	bytesIndex   int
	sessionIndex int
	funcType     reflect.Type
	funcValue    reflect.Value
}
type ActionRegister struct {
	websocketName        string
	websocketServiceType *websocketServiceType
}

var websocketServices = make(map[string]*websocketServiceType)
var regexWebsocketServices = make(map[string]*websocketServiceType)

var webSocketActionAuthChecker func(uint, *string, *string, *map[string]interface{}, interface{}) bool

// 注册Websocket服务
func RegisterWebsocket(authLevel uint, name string, updater *websocket.Upgrader,
	onOpen interface{},
	//onMessage func(*websocket.Conn, []byte),
	onClose interface{},
	decoder func(data interface{}) (action string, request *map[string]interface{}, err error),
	encoder func(action string, data interface{}) interface{}) *ActionRegister {

	s := new(websocketServiceType)
	s.authLevel = authLevel
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

	return &ActionRegister{websocketName: name, websocketServiceType: s}
}

func (ar *ActionRegister) RegisterAction(authLevel uint, actionName string, action interface{}) {
	a := new(websocketActionType)
	a.authLevel = authLevel
	a.funcType = reflect.TypeOf(action)
	if a.funcType != nil {
		a.parmsNum = a.funcType.NumIn()
		a.inIndex = -1
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
			}
		}
	}
	ar.websocketServiceType.actions[actionName] = a
}

func RegisterWebsocketActionAuthChecker(authChecker func(authLevel uint, url *string, action *string, request *map[string]interface{}, sess interface{}) bool) {
	webSocketActionAuthChecker = authChecker
}

func doWebsocketService(ws *websocketServiceType, request *http.Request, response *http.ResponseWriter, args *map[string]interface{}, headers *map[string]string, startTime *time.Time) {
	// 前置过滤器
	var result interface{} = nil
	for _, filter := range inFilters {
		result = filter(args, headers, request, response)
		if result != nil {
			break
		}
	}
	byteArgs, _ := json.Marshal(*args)
	byteHeaders, _ := json.Marshal(*headers)

	code := 200
	message := "OK"
	client, err := ws.updater.Upgrade(*response, request, nil)
	if err != nil {
		code = 500
		message = err.Error()
		(*response).WriteHeader(500)
	}

	if recordLogs {
		nowTime := time.Now()
		usedTime := float32(nowTime.UnixNano()-startTime.UnixNano()) / 1e6
		*startTime = nowTime
		log.Printf("WSOPEN	%s	%s	%.6f	%d	%s	%s	%s", request.RemoteAddr, request.RequestURI, usedTime, code, message, string(byteArgs), string(byteHeaders))
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

				var actionName string
				var messageData *map[string]interface{}
				if ws.decoder != nil {
					actionName, messageData, err = ws.decoder(*msg)
					if err != nil {
						log.Printf("ERROR	Read a bad message	%s	%s	%s", request.RemoteAddr, request.RequestURI, fmt.Sprint(*msg))
					}
				} else {
					actionName = ""
					mapMsg, isMap := (*msg).(map[string]interface{})
					if isMap {
						messageData = &mapMsg
					} else {
						messageData = &map[string]interface{}{"data": *msg}
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

				byteMsg, _ := json.Marshal(messageData)
				if webSocketActionAuthChecker != nil {
					if action.authLevel > 0 && webSocketActionAuthChecker(action.authLevel, &request.RequestURI, &actionName, messageData, sessionValue) == false {
						if recordLogs {
							log.Printf("WSREJECT	%s	%s	%s	%s	%d", request.RemoteAddr, request.RequestURI, actionName, string(byteMsg), action.authLevel)
						}
						(*response).WriteHeader(403)
						continue
					}
				}

				startTime := time.Now()
				err = doWebsocketAction(ws, action, client, messageData, sessionValue)
				if recordLogs {
					usedTime := time.Now().UnixNano() - startTime.UnixNano()
					if err == nil {
						log.Printf("WSACTION	%s	%s	%s	%.6f	%s", request.RemoteAddr, request.RequestURI, actionName, usedTime, string(byteMsg))
					} else {
						log.Printf("WSERROR	%s	%s	%s	%.6f	%s	%s", request.RemoteAddr, request.RequestURI, actionName, usedTime, string(byteMsg), err.Error())
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
				usedTime := float32(time.Now().UnixNano()-startTime.UnixNano()) / 1e6
				log.Printf("WSCLOSE	%s	%s	%.6f	%d	%s	%s	%s", request.RemoteAddr, request.RequestURI, usedTime, code, message, string(byteArgs), string(byteHeaders))
			}

		}
	}
}

func doWebsocketAction(ws *websocketServiceType, action *websocketActionType, client *websocket.Conn, data *map[string]interface{}, sess reflect.Value) error {
	var messageParms = make([]reflect.Value, action.parmsNum)
	if action.inType != nil {
		in := reflect.New(action.inType).Interface()
		err := mapstructure.WeakDecode(*data, in)
		if err != nil {
			return err
		}
		messageParms[action.inIndex] = reflect.ValueOf(in).Elem()
	}
	if action.sessionIndex >= 0 {
		messageParms[action.sessionIndex] = sess
	}
	if action.clientIndex >= 0 {
		messageParms[action.clientIndex] = reflect.ValueOf(client)
	}
	outs := action.funcValue.Call(messageParms)
	if ws.decoder != nil && len(outs) == 2 {
		b, err := json.Marshal(ws.encoder(outs[0].String(), outs[1].Interface()))
		if err != nil {
			return err
		}
		base.FixUpperCase(b)
		err = client.WriteMessage(websocket.TextMessage, b)
		if err != nil {
			return err
		}
	}
	return nil
}
