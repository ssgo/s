package tests

import (
	"fmt"
	"github.com/ssgo/config"
	"github.com/ssgo/u"
	"os"
	"testing"

	"github.com/ssgo/discover"
	"github.com/ssgo/s"
)

func TestRewrite(tt *testing.T) {
	t := s.T(tt)
	//os.Setenv("SERVICE_LOGFILE", os.DevNull)
	s.Register(0, "/echo", func(in struct{ S1, S2 string }, c *discover.Caller) string {
		fmt.Println("   #####", u.JsonP(in))
		return in.S1 + " " + in.S2
	})
	s.Register(0, "/echo/{s1}", func(in struct{ S1, S2 string }, c *discover.Caller) string {
		return in.S1 + " " + in.S2
	})
	s.Register(0, "/show/{s1}/{s2}", func(in struct{ S1, S2 string }, c *discover.Caller) string {
		return in.S1 + " " + in.S2
	})

	s.Rewrite("/r1", "/echo")
	s.Rewrite("/r2/(.+?)", "/echo/$1")

	s.Rewrite("/r3\\?name=(\\w+)", "/echo/$1")
	s.Rewrite("/r4", "http://localhost:18811/echo")
	s.Rewrite("/r5/(.+?)/(.+?)\\?(.+?)", "/echo/$1?s2=$2&$3")
	s.Rewrite("/r5/(.+?)/(.+?)", "/echo/$1?s2=$2")
	s.Rewrite("/r6/(.+?)/(.+?)", "/show/$1/$2")

	_ = os.Setenv("SERVICE_LISTEN", ":18811")
	config.ResetConfigEnv()
	s.Init()
	as := s.AsyncStart()
	defer as.Stop()

	r := as.Post("/echo?s1=a", s.Map{"s2": "b"}).String()
	t.Test(r == "a b", "echo", r)

	r = as.Post("/r1?s1=a", s.Map{"s2": "b"}).String()
	t.Test(r == "a b", "r1", r)

	r = as.Post("/r2/a", s.Map{"s2": "b"}).String()
	t.Test(r == "a b", "r2", r)

	r = as.Post("/r3?name=a", s.Map{"s2": "b"}).String()
	t.Test(r == "a b", "r3", r)

	res := as.Post("/r4?s1=a", s.Map{"s2": "b"})
	t.Test(res.Response != nil && res.Response.Header.Get("Location") == "http://localhost:18811/echo?s1=a", "r4", res.Response.Header.Get("Location"))

	r = as.Get("/echo/a?s2=b").String()
	t.Test(r == "a b", "/echo/a?s2=b", r)

	r = as.Get("/r5/a/b?name=jim&age=17").String()
	t.Test(r == "a b", "/r5/a/b?name=jim&age=17", r)

	r = as.Get("/r6/a/b").String()
	t.Test(r == "a b", "/r6/a/b", r)
}
