package tests

import (
	"github.com/ssgo/config"
	"testing"

	"github.com/ssgo/httpclient"
	"github.com/ssgo/s"
)

func Hello() string {
	return "Hello"
}

func T1estHttp1(tt *testing.T) {
	t := s.T(tt)

	config.ResetConfigEnv()
	s.ResetAllSets()
	s.Register(0, "/hello", Hello, "")
	as := s.AsyncStart()
	defer as.Stop()

	c := httpclient.GetClientH2C(1000)
	r := c.Get("http://" + as.Addr + "/hello")
	t.Test(r.Error == nil && r.String() == "Hello", "Hello", r.Error, r.String())
}
