package tests

import (
	".."
	"testing"
	"github.com/gorilla/websocket"
	"strings"
)

func TestEchoWS(tt *testing.T) {
	t := s.T(tt)

	s.ResetAllSets()
	s.RegisterWebsocket("/echoService/{token}/{roomId}", nil, OnEchoOpen, OnEchoClose, EchoDecoder)
	s.RegisterWebsocketAction("/echoService/{token}/{roomId}", "", OnEchoMessage)
	s.EnableLogs(false)

	serv := s.StartTestService()
	defer s.StopTestService()

	c, _, err := websocket.DefaultDialer.Dial(strings.Replace(serv.URL, "http", "ws", 1)+"/echoService/abc-123/99", nil)
	t.Test(err == nil, "Connect", err)

	r := make(map[string]interface{})
	err = c.ReadJSON(&r)
	t.Test(err == nil, "Read welcome", err)
	t.Test(r["action"] == "welcome" && r["token"] == "abc-123" && r["roomId"].(float64) == 99 && r["oldAge"].(float64) == 1, "Welcome", r, c, err)

	oldAge := 1
	for newAge := 10; newAge < 12; newAge++ {
		err = c.WriteJSON(s.Map{
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
	s.ResetAllSets()
	s.RegisterWebsocket("/echoService/{token}/{roomId}", nil, OnEchoOpen, OnEchoClose, EchoDecoder)
	s.RegisterWebsocketAction("/echoService/{token}/{roomId}", "", OnEchoMessage)
	s.EnableLogs(false)
	serv := s.StartTestService()
	defer s.StopTestService()
	b.StartTimer()

	//threadIndex := 0
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//threadIndex++

			//idx := threadIndex
			//logFileName := fmt.Sprint("/tmp/aa/",idx)
			//err := logToFile(logFileName, fmt.Sprintln(idx))
			//if err != nil {
			//	b.Error("logToFile", err)
			//	continue
			//}

			c, _, err := websocket.DefaultDialer.Dial(strings.Replace(serv.URL, "http", "ws", 1)+"/echoService/abc-123/99", nil)

			//err2 := logToFile(logFileName, fmt.Sprintln(" - ", idx, c!=nil, err))
			//if err2 != nil {
			//	b.Error("logToFile", err2)
			//	continue
			//}

			if c == nil || err != nil {
				b.Error("Conn error", err)
				continue
			}

			r := make(map[string]interface{})
			err = c.ReadJSON(&r)
			if err != nil {
				b.Error("Read welcome error", err)
				continue
			}

			oldAge := 1
			for newAge := 10; newAge < 210; newAge++ {
				err = c.WriteJSON(s.Map{
					"action": "echo",
					"age":    newAge,
				})

				if err != nil {
					b.Error("Send echo error", err)
					continue
				}
				err = c.ReadJSON(&r)
				if err != nil && r["action"] == "echo" && int(r["oldAge"].(float64)) == oldAge && int(r["newAge"].(float64)) == newAge {
					b.Error("Read echo error", err)
					continue
				}
				oldAge = newAge
			}
			err = c.Close()
			if err != nil {
				b.Error("Close error", err)
				continue
			}
		}
	})

	//time.Sleep(time.Second*1)
}

func BenchmarkWSEchoAction(b *testing.B) {
	b.StopTimer()
	s.ResetAllSets()
	s.RegisterWebsocket("/echoService/{token}/{roomId}", nil, OnEchoOpen, OnEchoClose, EchoDecoder)
	s.RegisterWebsocketAction("/echoService/{token}/{roomId}", "", OnEchoMessage)
	s.EnableLogs(false)
	serv := s.StartTestService()
	defer s.StopTestService()

	c, _, err := websocket.DefaultDialer.Dial(strings.Replace(serv.URL, "http", "ws", 1)+"/echoService/abc-123/99", nil)

	if c == nil || err != nil {
		b.Error("Conn error", err)
		return
	}

	r := make(map[string]interface{})
	err = c.ReadJSON(&r)
	if err != nil {
		b.Error("Read welcome error", err)
		return
	}
	b.StartTimer()

	oldAge := 1
	for newAge:=0; newAge<b.N; newAge++ {
		err = c.WriteJSON(s.Map{
			"action": "echo",
			"age":    newAge,
		})

		if err != nil {
			b.Error("Send echo error", err)
			continue
		}
		err = c.ReadJSON(&r)
		if err != nil && r["action"] != "echo" && int(r["oldAge"].(float64)) != oldAge && int(r["newAge"].(float64)) != newAge {
			b.Log(r)
			b.Error("Read echo error", err)
			continue
		}
		//oldAge = newAge
	}




	err = c.Close()
	if err != nil {
		b.Error("Close error", err)
	}

	//b.FailNow()
	//time.Sleep(time.Second*1)
}

//func logToFile(fileName, text string) error{
//	var f *os.File
//	var err error
//	if _, err = os.Stat(fileName); os.IsNotExist(err) {
//		f, err = os.Create(fileName)
//	}else{
//		f, err = os.OpenFile(fileName, os.O_RDWR|os.O_APPEND, 0666)
//	}
//	if err != nil {
//		return err
//	}
//
//	_, err = f.WriteString(text)
//	if err != nil {
//		return err
//	}
//
//	err = f.Close()
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
