package tests

import (
	"net/http"
	"github.com/gorilla/websocket"
	".."
)

type echo1Args struct {
	Aaa int
	Bbb string
	Ccc string
	Ddd float32
	Eee bool
	Fff interface{}
	Ggg string
}

type echo2Args struct {
	Aaa        int
	Bbb        string
	Ccc        string
	Ddd        float32
	Eee        bool
	Fff        interface{}
	Ggg        string
	FilterTag  string
	FilterTag2 int
}



func Echo1(in echo1Args, headers struct{ Cid string }) (code int, message string, data interface{}) {
	return 211, "OK", []interface{}{in, headers}
}

func Echo2(req *http.Request, in echo2Args) (code int, message string, data interface{}) {
	return 211, "OK", in
}

func Echo3(res http.ResponseWriter, in struct {
	Name string
}, req *http.Request) (code int, message string, data interface{}) {
	return 211, "OK", []interface{}{in.Name, req.RequestURI}
}

type echoWsSession struct {
	UserId   int
	UserName string
	RoomId   int
	//Lock sync.Mutex
	UserInfo struct {
		Age int
	}
}

func OnEchoOpen(in struct {
	Token  string
	RoomId int
}, client *websocket.Conn) *echoWsSession {
	client.EnableWriteCompression(true)
	sess := new(echoWsSession)
	sess.UserId = 100
	sess.UserName = "Sam"
	sess.UserInfo.Age = 1
	sess.RoomId = in.RoomId

	client.WriteJSON(s.Map{
		"action": "welcome",
		"token":  in.Token,
		"roomId": in.RoomId,
		"oldAge": sess.UserInfo.Age,
	})
	return sess
}
func OnEchoMessage(in struct {
	Action string
	Age    int
}, client *websocket.Conn, sess *echoWsSession) {
	//sess.Lock.Lock()
	client.WriteJSON(s.Map{
		"action": "echo",
		"oldAge": sess.UserInfo.Age,
		"newAge": in.Age,
	})
	//sess.Lock.Unlock()
	sess.UserInfo.Age = in.Age
}

func OnEchoClose(client *websocket.Conn, sess *echoWsSession) {
}

func EchoDecoder(srcData *interface{}) (string, s.Map, error) {
	dstData := (*srcData).(map[string]interface{})
	return dstData["action"].(string), dstData, nil
}

//func EchoWS(in struct {
//	HttpRequestPath string
//	client *websocket.Conn
//}){
//	log.Println(in.HttpRequestPath)
//	for {
//		_, message, err := in.client.ReadMessage()
//		if err != nil {
//			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
//				log.Printf("error: %v", err)
//			}
//			break
//		}
//		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
//		c.hub.broadcast <- message
//	}
//	return 211, "OK", []interface{}{in.Name, in.RedisPool, in.HttpRequestPath, in.HttpRequest.RequestURI}
//}
