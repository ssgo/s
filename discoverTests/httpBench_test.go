package main

import (
	"os"
	"testing"

	"github.com/ssgo/s"
)

func BenchmarkForHttpClient1(tb *testing.B) {
	benchmarkForHttpClient(tb, 1)
}

func BenchmarkForHttpClient2(tb *testing.B) {
	benchmarkForHttpClient(tb, 2)
}

func benchmarkForHttpClient(tb *testing.B, httpVersion int) {
	os.Setenv("SERVICE_LOGFILE", os.DevNull)
	tb.StopTimer()
	s.Register(0, "/s1", func() (out struct{ Name string }) {
		out.Name = "s1"
		return
	})
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
			d := struct{ Name string }{}
			r := as.Get("/s1")
			r.To(&d)
			if r.Error != nil || d.Name != "s1" {
				tb.Error("Discover Benchmark", r.Error, r.String(), r.Response)
			}
		}
	})
	tb.StopTimer()
}
