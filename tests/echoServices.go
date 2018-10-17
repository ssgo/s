package tests

import (
	"fmt"
	"net/http"

	".."
	"github.com/gorilla/websocket"
)

type echo1Args struct {
	Aaa int `check ^\d+$`
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

func Echo1(in echo1Args, headers *http.Header) (out struct {
	In      echo1Args
	Headers struct{ Cid string }
}) {
	//c.Call("lesson", "/getList", s.Map{"id": 100})
	out.In = in
	out.Headers.Cid = headers.Get("Cid")
	return
}

func Echo2(req *http.Request, in echo2Args) echo2Args {
	return in
}

func Echo3(res http.ResponseWriter, in struct {
	Name string
}, req *http.Request) []string {
	return []string{in.Name, req.RequestURI}
}

func Echo4(in s.Map) s.Map {
	return in
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

	client.WriteJSON(EchoEncoder("welcome", map[string]interface{}{
		"token":  in.Token,
		"roomId": in.RoomId,
		"oldAge": sess.UserInfo.Age,
	}))
	return sess
}

type echoAgeData struct {
	OldAge int
	NewAge int
}

func OnEchoMessage(in struct {
	Action string
	Age    int
}, client *websocket.Conn, sess *echoWsSession) (string, *echoAgeData) {
	oldAge := sess.UserInfo.Age
	sess.UserInfo.Age = in.Age
	return "echo", &echoAgeData{
		OldAge: oldAge,
		NewAge: in.Age,
	}
}

func OnEchoClose(client *websocket.Conn, sess *echoWsSession) {
}

func EchoDecoder(srcData interface{}) (string, *map[string]interface{}, error) {
	var a []interface{}
	var m map[string]interface{}
	var ok bool
	if a, ok = srcData.([]interface{}); ok {
		if m, ok = a[1].(map[string]interface{}); ok {
			return a[0].(string), &m, nil
		}
	}
	return "", nil, fmt.Errorf("in data err	%s", fmt.Sprint(srcData))
}

func EchoEncoder(action string, data interface{}) interface{} {
	return []interface{}{action, data}
}
