package tests

import (
	"net"
	"net/http"
	"testing"

	"github.com/ssgo/s/discover"
)

func TestBase(t *testing.T) {

	l, _ := net.Listen("tcp", ":18001")
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("OK"))
	})
	go func() {
		http.Serve(l, nil)
	}()

	discover.Start("127.0.0.1:18001", discover.Config{
		App: "s1",
		Calls: map[string]*discover.CallInfo{
			"s1": &discover.CallInfo{
				Headers: map[string]string{
					"Access-Token": "xxxx",
				},
				HttpVersion: 1,
			},
		},
	})

	//time.Sleep(100 * time.Millisecond)
	appClient := discover.AppClient{}
	node := appClient.Next("s1", nil)
	if node == nil {
		t.Error("node is nil ")
	}
	if node.Addr != "127.0.0.1:18001" {
		t.Error(node.Addr, " != 127.0.0.1:18001")
	}

	caller := discover.Caller{}
	result := caller.Get("s1", "/").String()
	if result != "OK" {
		t.Error(result, " != OK")
	}

	l.Close()
	discover.Stop()
	discover.Wait()
}
