package tests

import (
	".."
	"testing"
	"github.com/gorilla/websocket"
	"strings"
)

func TestEchoWS(tt *testing.T) {
	t := service.T(tt)

	service.ResetAllSets()
	service.RegisterWebsocket("/echoService/{token}/{roomId}", nil, OnEchoOpen, OnEchoClose, EchoDecoder)
	service.RegisterWebsocketAction("/echoService/{token}/{roomId}", "", OnEchoMessage)
	service.EnableLogs(false)

	serv := service.StartTestService()
	defer service.StopTestService()

	c, _, err := websocket.DefaultDialer.Dial(strings.Replace(serv.URL, "http", "ws", 1)+"/echoService/abc-123/99", nil)
	t.Test(err == nil, "Connect", err)

	r := make(map[string]interface{})
	err = c.ReadJSON(&r)
	t.Test(err == nil, "Read welcome", err)
	t.Test(r["action"] == "welcome" && r["token"] == "abc-123" && r["roomId"].(float64) == 99 && r["oldAge"].(float64) == 1, "Welcome", r, c, err)

	oldAge := 1
	for newAge:=10; newAge<200; newAge++ {
		err = c.WriteJSON(map[string]interface{}{
			"action": "echo",
			"age":    newAge,
		})
		t.Test(err == nil, "Send age", err)

		err = c.ReadJSON(&r)
		t.Test(err == nil, "Read age", err)

		t.Test(r["action"] == "echo" && int(r["oldAge"].(float64)) == oldAge && int(r["newAge"].(float64)) == newAge, "Echo age back", r, c, err)
		oldAge = newAge
	}
	c.Close()

	//time.Sleep(time.Millisecond * 10)
	//tt.FailNow()
}

func BenchmarkWSEcho(b *testing.B) {
	b.StopTimer()
	service.ResetAllSets()
	service.RegisterWebsocket("/echoService/{token}/{roomId}", nil, OnEchoOpen, OnEchoClose, EchoDecoder)
	service.RegisterWebsocketAction("/echoService/{token}/{roomId}", "", OnEchoMessage)
	service.EnableLogs(false)
	serv := service.StartTestService()
	defer service.StopTestService()
	b.StartTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c, _, err := websocket.DefaultDialer.Dial(strings.Replace(serv.URL, "http", "ws", 1)+"/echoService/abc-123/99", nil)
			if c == nil || err != nil {
				b.Error("Conn error", err)
			}
			r := make(map[string]interface{})
			err = c.ReadJSON(&r)
			if err != nil {
				b.Error("Read welcome error", err)
			}

			for newAge:=10; newAge<200; newAge++ {
				err = c.WriteJSON(map[string]interface{}{
					"action": "echo",
					"age":    newAge,
				})

				if err != nil {
					b.Error("Send echo error", err)
				}
				err = c.ReadJSON(&r)
				if err != nil {
					b.Error("Read echo error", err)
				}
			}
			err = c.Close()
			if err != nil {
				b.Error("Close error", err)
			}
		}
	})
}
