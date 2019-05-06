package main

import (
	"github.com/ssgo/u"
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
	_ = os.Setenv("LOG_FILE", os.DevNull)
	_ = os.Setenv("SERVICE_HTTPVERSION", u.String(httpVersion))
	tb.StopTimer()
	s.Register(0, "/s1", func() (out struct{ Name string }) {
		out.Name = "s1"
		return
	})
	as := s.AsyncStart()
	defer as.Stop()

	tb.StartTimer()
	tb.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			d := struct{ Name string }{}
			r := as.Get("/s1")
			_ = r.To(&d)
			if r.Error != nil || d.Name != "s1" {
				tb.Error("Discover Benchmark", r.Error, r.String(), r.Response)
			}
		}
	})
	tb.StopTimer()
}
