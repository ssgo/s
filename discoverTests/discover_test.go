package main

import (
	".."
	"github.com/ssgo/redis"
	"os"
	"testing"
)

func TestBase(tt *testing.T) {
	t := s.T(tt)

	redis.GetRedis("discover:15").DEL("tb")
	s.Register(1, "/c1", func(c *s.Caller) string {
		r := struct{ Name string }{}
		c.Get("tb", "/s1").To(&r)
		return r.Name
	})
	s.Register(2, "/s1", func() (out struct{ Name string }) {
		out.Name = "s1"
		return
	})
	os.Setenv("SERVICE_APP", "tb")
	os.Setenv("SERVICE_WEIGHT", "100")
	os.Setenv("SERVICE_ACCESSTOKENS", `{"aabbcc": 1, "aabbcc222": 2}`)
	os.Setenv("SERVICE_CALLS", `{"tb": {"accessToken": "aabbcc222", "timeout": 200}}`)
	as := s.AsyncStart()
	defer as.Stop()

	r := as.Get("/c1", "Access-Token", "aabbcc")
	t.Test(r.Error == nil && r.String() == "s1", "DC", r.Error, r.String())
}
