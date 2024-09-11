package tests

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/ssgo/s"
)

type Echo1Args struct {
	Aaa int `check ^\d+$`
	Bbb string
	Ccc string
	Ddd float32
	Eee bool
	Fff any
	Ggg string
}

type Echo2Args struct {
	Echo1Args
	FilterTag  string
	FilterTag2 int
}

func Echo1(in Echo1Args, headers struct{ CID string }) (out struct {
	//func Echo1(in Echo1Args, headers map[string]string) (out struct {
	In      Echo1Args
	Headers struct{ CID string }
}) {
	//c.Call("lesson", "/getList", s.Map{"id": 100})
	out.In = in
	out.Headers.CID = headers.CID
	return
}

func Echo2(req *http.Request, in Echo2Args) Echo2Args {
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

	_ = client.WriteJSON(EchoEncoder("welcome", map[string]any{
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

func EchoDecoder(srcData any) (string, map[string]any, error) {
	var a []any
	var m map[string]any
	var ok bool
	if a, ok = srcData.([]any); ok {
		if m, ok = a[1].(map[string]any); ok {
			return a[0].(string), m, nil
		}
	}
	return "", nil, fmt.Errorf("in data err	%s", fmt.Sprint(srcData))
}

func EchoEncoder(action string, data any) any {
	return []any{action, data}
}
