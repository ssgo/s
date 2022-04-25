package tests

import (
	"os"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/ssgo/s"
)

func TestEchoWS(tt *testing.T) {
	t := s.T(tt)

	_ = os.Setenv("LOG_FILE", os.DevNull)
	//_ = os.Setenv("service_httpVersion", "1")
	_ = os.Setenv("service_listen", ":,http")
	s.ResetAllSets()
	echoAR := s.RegisterWebsocket(0, "/echoService/{token}/{roomId}", nil, OnEchoOpen, OnEchoClose, EchoDecoder, EchoEncoder)
	echoAR.RegisterAction(0, "", OnEchoMessage)

	as := s.AsyncStart()
	defer as.Stop()

	c, _, err := websocket.DefaultDialer.Dial("ws://"+as.Addr+"/echoService/abc-123/99", nil)
	t.Test(err == nil, "Connect", err)

	r := make([]interface{}, 0)
	err = c.ReadJSON(&r)
	t.Test(err == nil, "Read welcome", err)
	action, ok := r[0].(string)
	t.Test(ok, "Read welcome", err)
	data, ok := r[1].(map[string]interface{})
	t.Test(ok, "Read welcome", err)

	t.Test(action == "welcome" && data["token"] == "abc-123" && data["roomId"].(float64) == 99 && data["oldAge"].(float64) == 1, "Welcome", r, c, err)

	oldAge := 1
	for newAge := 10; newAge < 12; newAge++ {
		err = c.WriteJSON(s.Arr{"echo", s.Map{"age": newAge}})
		t.Test(err == nil, "Send age", err)

		err = c.ReadJSON(&r)
		t.Test(err == nil, "Read age", err)
		action, ok := r[0].(string)
		t.Test(ok, "Read age", err)
		data, ok := r[1].(map[string]interface{})
		t.Test(ok, "Read age", err)

		t.Test(action == "echo" && int(data["oldAge"].(float64)) == oldAge && int(data["newAge"].(float64)) == newAge, "Echo age back", r, oldAge, newAge, err)
		oldAge = newAge
	}
	_ = c.Close()
}

func BenchmarkWSEcho(b *testing.B) {
	b.StopTimer()
	_ = os.Setenv("LOG_FILE", os.DevNull)
	_ = os.Setenv("service_httpVersion", "1")
	s.ResetAllSets()
	echoAR := s.RegisterWebsocket(0, "/echoService/{token}/{roomId}", nil, OnEchoOpen, OnEchoClose, EchoDecoder, EchoEncoder)
	echoAR.RegisterAction(0, "", OnEchoMessage)
	as := s.AsyncStart()
	defer as.Stop()
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

			c, _, err := websocket.DefaultDialer.Dial("ws://"+as.Addr+"/echoService/abc-123/99", nil)

			//err2 := logToFile(logFileName, fmt.Sprintln(" - ", idx, c!=nil, err))
			//if err2 != nil {
			//	b.Error("logToFile", err2)
			//	continue
			//}

			if c == nil || err != nil {
				b.Error("Conn error", err)
				continue
			}

			r := make([]interface{}, 0)
			err = c.ReadJSON(&r)
			if err != nil {
				b.Error("Read welcome error", err)
				continue
			}

			oldAge := 1
			for newAge := 10; newAge < 210; newAge++ {
				err = c.WriteJSON(s.Arr{"echo", s.Map{"age": newAge}})

				if err != nil {
					b.Error("Send echo error", err)
					continue
				}
				err = c.ReadJSON(&r)
				action := r[0].(string)
				data := r[1].(map[string]interface{})
				if err != nil && action == "echo" && int(data["oldAge"].(float64)) == oldAge && int(data["newAge"].(float64)) == newAge {
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
	_ = os.Setenv("LOG_FILE", os.DevNull)
	_ = os.Setenv("service_httpVersion", "1")
	s.ResetAllSets()
	echoAR := s.RegisterWebsocket(0, "/echoService/{token}/{roomId}", nil, OnEchoOpen, OnEchoClose, EchoDecoder, EchoEncoder)
	echoAR.RegisterAction(0, "", OnEchoMessage)
	as := s.AsyncStart()
	defer as.Stop()

	c, _, err := websocket.DefaultDialer.Dial("ws://"+as.Addr+"/echoService/abc-123/99", nil)

	if c == nil || err != nil {
		b.Error("Conn error", err)
		return
	}

	r := make([]interface{}, 0)
	err = c.ReadJSON(&r)
	if err != nil {
		b.Error("Read welcome error", err)
		return
	}
	b.StartTimer()

	oldAge := 1
	for newAge := 0; newAge < b.N; newAge++ {
		err = c.WriteJSON(s.Arr{"echo", s.Map{"age": newAge}})

		if err != nil {
			b.Error("Send echo error", err)
			continue
		}
		err = c.ReadJSON(&r)
		action := r[0].(string)
		data := r[1].(map[string]interface{})
		if err != nil && action != "echo" && int(data["oldAge"].(float64)) != oldAge && int(data["newAge"].(float64)) != newAge {
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
