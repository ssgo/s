package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/ssgo/s"
	"os"
)

type wsSession struct {
	UserId   int
	UserName string
	RoomId   int
	//Lock sync.Mutex
	UserInfo struct {
		Age int
	}
}

type ageData struct {
	OldAge int
	NewAge int
}

func OnWsOpen(in struct {
	Token  string
	RoomId int
}, client *websocket.Conn) *wsSession {
	client.EnableWriteCompression(true)
	sess := new(wsSession)
	sess.UserId = 100
	sess.UserName = "Sam"
	sess.UserInfo.Age = 1
	sess.RoomId = in.RoomId
	fmt.Println("===OnWsOpen===")

	client.WriteJSON(WsEncoder("welcome", map[string]interface{}{
		"token":  in.Token,
		"roomId": in.RoomId,
		"oldAge": sess.UserInfo.Age,
	}))
	return sess
}

func OnWsMessage(in struct {
	Action string
	Age    int
}, client *websocket.Conn, sess *wsSession) (string, *ageData) {
	oldAge := sess.UserInfo.Age
	sess.UserInfo.Age = in.Age
	fmt.Println("===OnWsMessage===")
	return "echo", &ageData{
		OldAge: oldAge,
		NewAge: in.Age,
	}
}

func OnWsClose(client *websocket.Conn, sess *wsSession) {
	fmt.Println("===OnWsClose===")
}

func WsDecoder(srcData interface{}) (string, *map[string]interface{}, error) {
	fmt.Println("===WsDecoder===")
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

func WsEncoder(action string, data interface{}) interface{} {
	fmt.Println("===WsEncoder===")

	return []interface{}{action, data}
}

func main() {
	app := "w1"
	listen := ":8308"
	os.Setenv("SERVICE_LOGFILE", os.DevNull)
	os.Setenv("SERVICE_APP", app)
	os.Setenv("SERVICE_LISTEN", listen)
	s.ResetAllSets()
	wsAR := s.RegisterWebsocket(0, "/service/{token}/{roomId}", nil,
		OnWsOpen, OnWsClose, WsDecoder, WsEncoder)
	wsAR.RegisterAction(0, "", OnWsMessage)

	as := s.AsyncStart1()
	defer as.Stop()
	fmt.Println("websocket dial")
	c, _, err := websocket.DefaultDialer.Dial("ws://"+as.Addr+"/service/abc-123/99", nil)
	if err != nil {
		fmt.Println("dial err is:", err)
	}
	r := make([]interface{}, 0)
	err = c.ReadJSON(&r)
	fmt.Println("r is:", r, "err is:", err)

	for newAge := 10; newAge < 15; newAge++ {
		fmt.Println("Send age loop:")
		err = c.WriteJSON(s.Arr{"echo", s.Map{"age": newAge}})
		if err != nil {
			fmt.Println("Send err is:", err)
		}
		err = c.ReadJSON(&r)
		fmt.Println("r is:", r, "err is:", err)
	}
	c.Close()
}
