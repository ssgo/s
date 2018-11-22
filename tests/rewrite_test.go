package tests

import (
	"os"
	"testing"

	"github.com/ssgo/s"
	"github.com/ssgo/s/base"
	"github.com/ssgo/s/discover"
)

func TestRewrite(tt *testing.T) {
	t := s.T(tt)

	s.Register(0, "/echo", func(in struct{ S1, S2 string }, c *discover.Caller) string {
		return in.S1 + " " + in.S2
	})
	s.Register(0, "/echo/{s1}", func(in struct{ S1, S2 string }, c *discover.Caller) string {
		return in.S1 + " " + in.S2
	})

	s.Rewrite("/r1", "/echo")
	s.Rewrite("/r2/(.+?)", "/echo/$1")
	s.Rewrite("/r3\\?name=(\\w+)", "/echo/$1")
	s.Rewrite("/r4", "http://localhost:18811/echo")

	os.Setenv("SERVICE_LISTEN", ":18811")
	base.ResetConfigEnv()
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

	r = as.Post("/r4?s1=a", s.Map{"s2": "b"}).String()
	t.Test(r == "a b", "r4", r)
}
