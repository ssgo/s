package main

import (
	"github.com/ssgo/s"
	"testing"
)

func BenchmarkForDiscover(tb *testing.B) {
	c := s.GetClient1()
	tb.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r := c.Get("http://127.0.0.1:8080/Sam", "Access-Token", "aabbcc")
			if r.Error != nil || r.String() != `{"fullName":"Sam Lee"}` {
				tb.Error("Discover Benchmark", r.Error, r.String(), r.Response)
			}
		}
	})
}
