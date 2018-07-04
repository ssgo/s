package main

import (
	".."
	"github.com/ssgo/base"
	"github.com/ssgo/discover"
	"github.com/ssgo/redis"
	"os"
	"testing"
)

func TestBase(tt *testing.T) {
	t := s.T(tt)

	redis.GetRedis("discover:15").DEL("a1")

	s.Register(2, "/dc/s1", func() (out struct{ Name string }) {
		out.Name = "s1"
		return
	})

	s.Register(1, "/dc/c1", func(c *discover.Caller) string {
		r := struct{ Name string }{}
		c.Get("a1", "/dc/s1").To(&r)
		return r.Name
	})

	s.Proxy("/dc1/s1", "a1", "/dc/s1")
	s.Proxy("/dc2/(.+?)", "a1", "/dc/$1")

	//s.Rewrite("/rrr/123", "/dc/c1")
	//s.Rewrite("/r2/(.+?)/aa", "/dc/$1")
	//s.Rewrite("/r3\\?name=(\\w+)", "/dc/$1")
	//s.Rewrite1("/bd", "http://www.hfjy.com/")

	os.Setenv("SERVICE_APP", "a1")
	os.Setenv("SERVICE_WEIGHT", "100")
	os.Setenv("SERVICE_ACCESSTOKENS", `{"aabbcc": 1, "aabbcc222": 2}`)
	os.Setenv("SERVICE_CALLS", `{"a1": {"accessToken": "aabbcc222", "timeout": 200}}`)
	base.ResetConfigEnv()
	as := s.AsyncStart()
	defer as.Stop()

	r := as.Get("/dc/c1", "Access-Token", "aabbcc")
	t.Test(r.Error == nil && r.String() == "s1", "DC", r.Error, r.String())

	//r = as.Get("/rrr/123?a=1", "Access-Token", "aabbcc")
	//t.Test(r.Error == nil && r.String() == "s1", "DC by rewrite", r.Error, r.String())
	//
	//r = as.Get("/r2/c1/aa", "Access-Token", "aabbcc")
	//t.Test(r.Error == nil && r.String() == "s1", "DC by rewrite 2", r.Error, r.String())
	//
	//r = as.Get("/r3?name=c1", "Access-Token", "aabbcc")
	//t.Test(r.Error == nil && r.String() == "s1", "DC by rewrite 3", r.Error, r.String())

	r1 := as.Get("/dc1/s1", "Access-Token", "aabbcc").Map()
	t.Test(r.Error == nil && r1["name"] == "s1", "DC by proxy", r.Error, r1)

	r1 = as.Get("/dc2/s1", "Access-Token", "aabbcc").Map()
	t.Test(r.Error == nil && r1["name"] == "s1", "DC by proxy 2", r.Error, r1)

	//r = as.Get("/bd")
	//t.Test(r.Error == nil && strings.Contains(r.String(), "baidu"), "DC by rewrite baidu", r.Error, r.String())
}
