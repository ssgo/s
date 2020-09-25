package tests

import (
	"net/http"
	"os"
	"testing"

	"github.com/ssgo/s"
)

type StatisticContext struct {
	s.Context
}

func TestStatistic(tt *testing.T) {
	_ = os.Setenv("service_StatisticTime", "true")
	_ = os.Setenv("service_StatisticTimeInterval", "100")
	//_ = os.Setenv("service_NoLogGets", "true")

	s.ResetAllSets()
	s.SetInject(&StatisticContext{})
	s.Restful(0, "GET", "/hello", func(in struct{ Name string }, request *http.Request, response *s.Response, ctx *StatisticContext) string {
		return "Hello World!"
	})
	as := s.AsyncStart()

	for i := 0; i < 100; i++ {
		as.Get("/hello?name=aaa")
	}
	as.Stop()
}
