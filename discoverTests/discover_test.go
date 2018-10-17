package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	".."
	"github.com/gorilla/websocket"
	"github.com/ssgo/s/base"
	"github.com/ssgo/s/discover"
	"github.com/ssgo/s/redis"
)

func TestBase(tt *testing.T) {
	t := s.T(tt)

	dc := redis.GetRedis("discover:15")
	dc.DEL("a1")

	s.Register(2, "/dc/s1", func() (out struct{ Name string }) {
		out.Name = "s1"
		return
	})

	s.Register(1, "/dc/c1", func(c *discover.Caller) string {
		r := struct{ Name string }{}
		c.Get("a1", "/dc/s1").To(&r)
		return r.Name
	})

	i := 0
	s.Register(2, "/dc/s2", func(response http.ResponseWriter) string {
		i++
		if i%2 == 1 {
			response.WriteHeader(502)
			return ""
		}
		response.WriteHeader(200)
		return "OK"
	})

	ws := s.RegisterWebsocket(1, "/dc/ws", nil, nil, nil, nil, nil)
	ws.RegisterAction(0, "hello", func(in struct{ Name string }) (out struct{ Name string }) {
		out.Name = in.Name + "!"
		return
	})

	s.Proxy("/dc1/s1", "a1", "/dc/s1")
	s.Proxy("/proxy/(.+?)", "a1", "/dc/$1")

	os.Setenv("SERVICE_APP", "a1")
	os.Setenv("SERVICE_WEIGHT", "100")
	os.Setenv("SERVICE_ACCESSTOKENS", `{"aabbcc": 1, "aabbcc222": 2}`)
	os.Setenv("SERVICE_CALLS", `{"a1": {"accessToken": "aabbcc222", "httpVersion": 1}}`)
	base.ResetConfigEnv()
	as := s.AsyncStart1()

	addr2 := "127.0.0.1:" + strings.Split(as.Addr, ":")[1]
	dc.HSET("a1", addr2, 1)
	dc.Do("PUBLISH", "CH_a1", fmt.Sprintf("%s %d", addr2, 1))

	defer as.Stop()

	r0 := as.Get("/dc/s1", "Access-Token", "aabbcc222")
	t.Test(r0.Error == nil && r0.String() == "{\"name\":\"s1\"}", "Service", r0.Error, r0.String())

	r0 = as.Get("/dc/c1", "Access-Token", "aabbcc")
	t.Test(r0.Error == nil && r0.String() == "s1", "DC", r0.Error, r0.String())

	r1 := as.Get("/dc1/s1", "Access-Token", "aabbcc").Map()
	t.Test(r1["name"] == "s1", "DC by proxy", r1)

	r1 = as.Get("/proxy/s1", "Access-Token", "aabbcc").Map()
	t.Test(r1["name"] == "s1", "DC by proxy 2", r1)

	r2 := as.Get("/proxy/s2", "Access-Token", "aabbcc")
	fmt.Println(r2.Error)
	t.Test(r2.String() == "OK", "DC by proxy 3", r2)

	c, _, err := websocket.DefaultDialer.Dial("ws://"+addr2+"/proxy/ws", nil)
	t.Test(err == nil, "Connect", err)
	r := map[string]string{}
	err = c.WriteJSON(s.Map{"action": "hello", "name": "aaa"})
	t.Test(err == nil, "send hello", err)
	err = c.ReadJSON(&r)
	t.Test(err == nil || r["action"] != "hello" || r["name"] != "aaa!", "read hello", err, r)
	c.Close()
}
