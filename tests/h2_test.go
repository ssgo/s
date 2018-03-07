package tests

import (
	".."
	"testing"
)

func Hello() string {
	return "Hello"
}

func TestHttp1(tt *testing.T) {
	t := s.T(tt)

	s.ResetAllSets()
	s.Register(0, "/", Hello)
	as := s.AsyncStart()
	defer as.Stop()

	c := s.GetClient()
	r := c.Get("http://" + as.Addr)
	t.Test(r.Error == nil && r.String() == "Hello", "Hello", r.Error, r.String())
}
