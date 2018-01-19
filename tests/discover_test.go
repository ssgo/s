package tests

import (
	"testing"
	".."
	"os"
	"fmt"
)

func S1() (out struct{ Name string }) {
	out.Name = "s1"
	return
}

func C1(c *s.Caller) string {
	r := struct{ Name string }{}
	c.Get("ta", "/s1").To(&r)
	return r.Name
}

func TestBase(tt *testing.T) {
	t := s.T(tt)

	s.Register(1, "/c1", C1)
	s.Register(2, "/s1", S1)
	os.Setenv("SERVICE_APP", "ta")
	os.Setenv("SERVICE_WEIGHT", "100")
	os.Setenv("SERVICE_ACCESSTOKENS", `{"aabbcc": 1, "aabbcc222": 2}`)
	os.Setenv("SERVICE_CALLS", `{"ta": {"accessToken": "aabbcc222", "timeout": 200}}`)
	as := s.AsyncStart()
	defer as.Stop()

	r := as.Get("/c1", "Access-Token", "aabbcc")
	t.Test(r.Error == nil && r.String() == "s1", "DC", r.Error, r.String())
}

func BenchmarkForHttpClient1(tb *testing.B) {
	benchmarkForHttpClient(tb, 1)
}

func BenchmarkForHttpClient2(tb *testing.B) {
	benchmarkForHttpClient(tb, 2)
}

func benchmarkForHttpClient(tb *testing.B, httpVersion int) {
	os.Setenv("SERVICE_LOGFILE", os.DevNull)
	tb.StopTimer()
	s.Register(0, "/s1", S1)
	var as *s.AsyncServer
	if httpVersion == 1 {
		as = s.AsyncStart1()
	} else {
		as = s.AsyncStart()
	}
	defer as.Stop()

	tb.StartTimer()
	tb.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r := as.Get("/s1")
			if r.Error != nil || r.String() != "s1" {
				tb.Error("Discover Benchmark", r.Error, r.String())
			}
		}
	})
	tb.StopTimer()
}

func BenchmarkForDiscover1(tb *testing.B) {
	benchmarkForDiscover(tb, 1)
}

func BenchmarkForDiscover2(tb *testing.B) {
	benchmarkForDiscover(tb, 2)
}

func benchmarkForDiscover(tb *testing.B, httpVersion int) {
	os.Setenv("SERVICE_LOGFILE", os.DevNull)
	tb.StopTimer()

	postfix := fmt.Sprint(httpVersion)
	s.Register(1, "/c1", func(c *s.Caller) string {
		r := struct{ Name string }{}
		c.Get("ta"+postfix, "/s1").To(&r)
		return r.Name
	})
	s.Register(2, "/s1", S1)
	os.Setenv("SERVICE_APP", "ta"+postfix)
	os.Setenv("SERVICE_WEIGHT", "100")
	os.Setenv("SERVICE_ACCESSTOKENS", `{"aabbcc": 1, "aabbcc222": 2}`)
	os.Setenv("SERVICE_CALLS", `{"ta`+postfix+`": {"accessToken": "aabbcc222", "timeout": 200, "httpVersion": `+postfix+`}}`)
	var as *s.AsyncServer
	if httpVersion == 1 {
		as = s.AsyncStart1()
	} else {
		as = s.AsyncStart()
	}

	tb.StartTimer()
	tb.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r := as.Get("/c1", "Access-Token", "aabbcc")
			if r.Error != nil || r.String() != "s1" {
				tb.Error("Discover Benchmark", r.Error, r.String())
			}
		}
	})
	tb.StopTimer()
	as.Stop()
}
