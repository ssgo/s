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
	openHeadersIndex  int
	openFuncType      reflect.Type
	openFuncValue     reflect.Value
	sessionType       reflect.Type
	closeParmsNum     int
	closeClientIndex  int
	closeRequestIndex int
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

var webSocketActionAuthChecker func(uint, *string, *string, *map[string]interface{}, *http.Request, interface{}) bool

// 注册Websocket服务
func RegisterWebsocket(authLevel uint, path string, updater *websocket.Upgrader,
	onOpen interface{},
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
		s.openClientIndex = -1
		s.openRequestIndex = -1
		s.openFuncValue = reflect.ValueOf(onOpen)
		for i := 0; i < s.openParmsNum; i++ {
			t := s.openFuncType.In(i)
			if t.Kind() == reflect.Struct {
				if s.openInType == nil {
					s.openInIndex = i
					s.openInType = t
				}
			} else if t.String() == "*http.Request" {
				s.openRequestIndex = i
			} else if t.String() == "*http.Header" {
				s.openHeadersIndex = i
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
			}
		}
	}

	finder, err := regexp.Compile("\\{(.*?)\\}")
	if err == nil {
		keyName := regexp.QuoteMeta(path)
		finds := finder.FindAllStringSubmatch(path, 20)
		for _, found := range finds {
			keyName = strings.Replace(keyName, regexp.QuoteMeta(found[0]), "(.*?)", 1)
			s.pathArgs = append(s.pathArgs, found[1])
		}
		if len(s.pathArgs) > 0 {
			s.pathMatcher, _ = regexp.Compile("^" + keyName + "$")
			if err != nil {
				log.Print("RegisterWebsocket	Compile	", err)
			}
			regexWebsocketServices[path] = s
		}
	}
	if s.pathMatcher == nil {
		websocketServices[path] = s
	}

	return &ActionRegister{websocketName: path, websocketServiceType: s}
}

func (ar *ActionRegister) RegisterAction(authLevel uint, actionName string, action interface{}) {
	a := new(websocketActionType)
	a.authLevel = authLevel
	a.funcType = reflect.TypeOf(action)
	if a.funcType != nil {
		a.parmsNum = a.funcType.NumIn()
		a.inIndex = -1
		a.sessionIndex = -1
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

func SetActionAuthChecker(authChecker func(authLevel uint, url *string, action *string, in *map[string]interface{}, request *http.Request, sess interface{}) bool) {
	webSocketActionAuthChecker = authChecker
}

func doWebsocketService(ws *websocketServiceType, request *http.Request, response *Response, args *map[string]interface{}, headers *map[string]string, startTime *time.Time) {
	byteArgs, _ := json.Marshal(*args)
	byteHeaders, _ := json.Marshal(*headers)

	message := "OK"
	client, err := ws.updater.Upgrade(response.writer, request, nil)
	if err != nil {
		message = err.Error()
		response.WriteHeader(500)
	}

	if recordLogs {
		nowTime := time.Now()
		usedTime := float32(nowTime.UnixNano()-startTime.UnixNano()) / 1e6
		*startTime = nowTime
		log.Printf("WSOPEN	%s	%s	%s	%s	%.6f	%d	%s	%s	%s	%s", getRealIp(request), request.Host, request.Method, request.RequestURI, usedTime, response.status, message, string(byteArgs), string(byteHeaders), request.Proto)
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
			if ws.openHeadersIndex >= 0 {
				openParms[ws.openRequestIndex] = reflect.ValueOf(&request.Header)
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
					log.Printf("ERROR	Read a bad message	%s	%s	%s", getRealIp(request), request.RequestURI, fmt.Sprint(*msg))
				}
			} else {
				actionName = ""
				mapMsg, isMap := (*msg).(map[string]interface{})
				if isMap {
					messageData = &mapMsg
					if (*messageData)["action"] != "" {
						actionName = base.String((*messageData)["action"])
					}
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

			printableMsg, _ := json.Marshal(messageData)
			if webSocketActionAuthChecker != nil {
				if action.authLevel > 0 && webSocketActionAuthChecker(action.authLevel, &request.RequestURI, &actionName, messageData, request, sessionValue) == false {
					if recordLogs {
						log.Printf("WSREJECT	%s	%s	%s	%s	%d", getRealIp(request), request.RequestURI, actionName, string(printableMsg), action.authLevel)
					}
					response.WriteHeader(403)
					continue
				}
			}

			startTime := time.Now()
			err = doWebsocketAction(ws, actionName, action, client, request, messageData, sessionValue)
			if recordLogs {
				//usedTime := time.Now().UnixNano() - startTime.UnixNano()
				usedTime := float32(time.Now().UnixNano()-startTime.UnixNano()) / 1e6
				if err == nil {
					log.Printf("WSACTION	%s	%s	%s	%.6f	%s", getRealIp(request), request.RequestURI, actionName, usedTime, string(printableMsg))
				} else {
					log.Printf("WSERROR	%s	%s	%s	%.6f	%s	%s", getRealIp(request), request.RequestURI, actionName, usedTime, string(printableMsg), err.Error())
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
			if ws.closeRequestIndex >= 0 {
				closeParms[ws.closeRequestIndex] = reflect.ValueOf(request)
			}
			ws.closeFuncValue.Call(closeParms)
		}

		if recordLogs {
			usedTime := float32(time.Now().UnixNano()-startTime.UnixNano()) / 1e6
			log.Printf("WSCLOSE	%s	%s	%s	%s	%.6f	%s	%s	%s	%s", getRealIp(request), request.Host, request.Method, request.RequestURI, usedTime, message, string(byteArgs), string(byteHeaders), request.Proto)
		}

	}
}

func doWebsocketAction(ws *websocketServiceType, actionName string, action *websocketActionType, client *websocket.Conn, request *http.Request, data *map[string]interface{}, sess reflect.Value) error {
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
	for i, parm := range messageParms {
		if parm.Kind() == reflect.Invalid {
			st := action.funcType.In(i)
			isset := false
			if st.Kind() == reflect.Struct || (st.Kind() == reflect.Ptr && st.Elem().Kind() == reflect.Struct) {
				sessObj := GetSessionInject(request, st)
				if sessObj != nil {
					messageParms[i] = reflect.ValueOf(sessObj)
					isset = true
				} else {
					injectObj := GetInject(st)
					if injectObj != nil {
						messageParms[i] = reflect.ValueOf(injectObj)
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
	if len(outs) > 0 {
		outAction := actionName
		var outData interface{}
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
				mapstructure.WeakDecode(outData, &outDataMap)
			} else {
				outDataMap = map[string]interface{}{}
				outDataMap["data"] = outData
			}
			outDataMap["action"] = outAction
			outBytes, err = json.Marshal(outDataMap)
		}

		if err != nil {
			return err
		}
		base.FixUpperCase(outBytes)
		err = client.WriteMessage(websocket.TextMessage, outBytes)
		if err != nil {
			return err
		}
	}

	return nil
}
