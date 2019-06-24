package tests

import (
	"errors"
	"fmt"
	"github.com/ssgo/u"
	"net/http"
	"os"
	"testing"

	"github.com/ssgo/s"
)

func TestEchos(tt *testing.T) {
	t := s.T(tt)

	s.ResetAllSets()
	s.Register(0, "/echo1", Echo1)
	s.Register(0, "/echo2", Echo2)
	s.Register(0, "/echo3", Echo3)
	s.Register(0, "/echo4", Echo4)

	_ = os.Setenv("SERVICE_LOGFILE", os.DevNull)
	as := s.AsyncStart()
	defer as.Stop()

	datas := as.Post("/echo1?aaa=11&bbb=_o_", s.Map{
		"ccc": "ccc",
		"DDD": 101.123,
		"eEe": true,
		"fff": nil,
		"ggg": "223",
	}, "CID", "test-client").Map()

	d1, ok := datas["in"].(map[string]interface{})
	t.Test(ok, "[Echo1] Data2", datas)
	d2, ok := datas["headers"].(map[string]interface{})
	t.Test(ok, "[Echo1] Data3", datas)
	t.Test(d1["aaa"].(float64) == 11 && d1["bbb"] == "_o_" && d1["ddd"] == 101.123 && d1["eee"] == true && d1["fff"] == nil, "[Echo1] In", datas)
	t.Test(d2["CID"] == "test-client", "[Echo1] Headers", datas)

	d := as.Post("/echo2?aaa=11&bbb=_o_", s.Map{
		"ccc": "ccc",
		"DDD": 101.123,
		"eEe": true,
		"fff": nil,
		"ggg": 223,
	}).Map()

	t.Test(u.Float64(d["aaa"]) == 11 && d["bbb"] == "_o_" && d["ddd"] == 101.123 && d["eee"] == true && d["fff"] == nil, "[Echo2] Data2", d)

	a := as.Post("/echo3?a=1", s.Map{"name": "Star"}).Arr()
	t.Test(ok, "[Echo3] Data1", a)
	t.Test(a[0] == "Star", "[Echo3] Data2", a)
	t.Test(a[1] == "/echo3?a=1", "[Echo3] Data3", a)

	d4 := as.Get("/echo4?aaa=11").Map()
	t.Test(d4["aaa"] == "11", "[Echo4]", d4)
}

func TestFilters(tt *testing.T) {
	t := s.T(tt)
	s.ResetAllSets()
	s.Register(0, "/echo2", Echo2)

	_ = os.Setenv("SERVICE_LOGFILE", os.DevNull)
	as := s.AsyncStart()
	defer as.Stop()

	d := as.Post("/echo2?aaa=11&bbb=_o_", s.Map{"ccc": "ccc"}).Map()
	t.Test(d["filterTag"] == "", "[Test InFilter 1] Response", d)

	s.SetInFilter(func(in *map[string]interface{}, request *http.Request, response *http.ResponseWriter) interface{} {
		(*in)["filterTag"] = "Abc"
		(*in)["filterTag2"] = 1000
		return nil
	})
	d = as.Post("/echo2?aaa=11&bbb=_o_", s.Map{"ccc": "ccc"}).Map()
	t.Test(d["filterTag"] == "Abc" && d["filterTag2"].(float64) == 1000, "[Test InFilter 2] Response", d)

	s.SetOutFilter(func(in *map[string]interface{}, request *http.Request, response *http.ResponseWriter, result interface{}) (interface{}, bool) {
		data := result.(echo2Args)
		data.FilterTag2 = data.FilterTag2 + 100
		return data, false
	})

	d = as.Post("/echo2?aaa=11&bbb=_o_", s.Map{"ccc": "ccc"}).Map()
	t.Test(d["filterTag"] == "Abc" && d["filterTag2"].(float64) == 1100, "[Test OutFilters 1] Response", d)

	s.SetOutFilter(func(in *map[string]interface{}, request *http.Request, response *http.ResponseWriter, result interface{}) (interface{}, bool) {
		data := result.(echo2Args)
		//fmt.Println(" ***************", data.FilterTag2+100)
		return s.Map{
			"filterTag":  (*in)["filterTag"],
			"filterTag2": data.FilterTag2 + 100,
		}, true
	})

	d = as.Post("/echo2?aaa=11&bbb=_o_", s.Map{"ccc": "ccc"}).Map()
	t.Test(d["filterTag"] == "Abc" && d["filterTag2"].(float64) == 1200, "[Test OutFilters 2] Response", d)

	s.SetInFilter(func(in *map[string]interface{}, request *http.Request, response *http.ResponseWriter) interface{} {
		return echo2Args{
			FilterTag:  (*in)["filterTag"].(string),
			FilterTag2: (*in)["filterTag2"].(int) + 100,
		}
	})
	d = as.Post("/echo2?aaa=11&bbb=_o_", s.Map{"ccc": "ccc"}).Map()
	t.Test(d["filterTag"] == "Abc" && d["filterTag2"].(float64) == 1300, "[Test InFilter 3] Response", d)
}

func TestAuth(tt *testing.T) {
	t := s.T(tt)
	s.ResetAllSets()
	s.Register(0, "/echo0", Echo2)
	s.Register(1, "/echo1", Echo2)
	s.Register(2, "/echo2", Echo2)

	s.SetAuthChecker(func(authLevel int, url *string, in *map[string]interface{}, request *http.Request) bool {
		token := request.Header.Get("Token")
		switch authLevel {
		case 1:
			return token == "aaa" || token == "bbb"
		case 2:
			return token == "bbb"
		}
		return false
	})

	as := s.AsyncStart()
	defer as.Stop()
	_ = os.Setenv("SERVICE_LOGFILE", os.DevNull)

	r := as.Get("/echo0")
	t.Test(r.Response.StatusCode == 200, "Test0", r.Response.StatusCode)

	r = as.Get("/echo1")
	t.Test(r.Response.StatusCode == 403, "Test1", r.Response.StatusCode)

	r = as.Get("/echo1", "Token", "aaa")
	t.Test(r.Response.StatusCode == 200, "Test1", r.Response.StatusCode)

	r = as.Get("/echo2")
	t.Test(r.Response.StatusCode == 403, "Test1", r.Response.StatusCode)

	r = as.Get("/echo1", "Token", "xxx")
	t.Test(r.Response.StatusCode == 403, "Test1", r.Response.StatusCode)

	r = as.Get("/echo1", "Token", "bbb")
	t.Test(r.Response.StatusCode == 200, "Test1", r.Response.StatusCode)

	r = as.Get("/echo2", "Token", "bbb")
	t.Test(r.Response.StatusCode == 200, "Test1", r.Response.StatusCode)
}

func panicFunc() {
	panic(errors.New("s panic test"))
}

func TestPanic(tt *testing.T) {
	t := s.T(tt)
	s.ResetAllSets()
	s.Register(0, "/panic_test", panicFunc)
	as := s.AsyncStart()
	defer as.Stop()
	_ = os.Setenv("SERVICE_LOGFILE", os.DevNull)
	r := as.Get("/panic_test")
	t.Test(r.Response.StatusCode == s.ResponseCodePanicError, "Response status code test")
}

func TestSetErrorHandle(tt *testing.T) {
	t := s.T(tt)
	s.ResetAllSets()
	s.Register(0, "/panic_test", panicFunc)
	s.SetErrorHandle(func(err interface{}, req *http.Request, rsp *http.ResponseWriter) interface{} {
		out := s.Map{"message": "defined", "code": 30889, "panic": fmt.Sprintf("%s", err)}
		_, _ = (*rsp).Write([]byte(u.String(out)))
		return out
	})
	as := s.AsyncStart()
	defer as.Stop()
	_ = os.Setenv("SERVICE_LOGFILE", os.DevNull)
	r := as.Get("/panic_test")
	panicArr := r.Map()
	//fmt.Println(panicArr)
	t.Test(r.Response.StatusCode == 200, "Response status code test")
	t.Test(panicArr["message"] == "defined" && panicArr["panic"] == "s panic test" && u.Int(panicArr["code"]) == 30889, "response test")
}

func BenchmarkEchosForStructWithLog(tb *testing.B) {
	tb.StopTimer()
	s.ResetAllSets()
	s.Register(0, "/echo1", Echo1)

	as := s.AsyncStart()
	defer as.Stop()

	as.Post("/echo1?aaa=11&bbb=_o_", s.Map{})

	tb.StartTimer()

	tb.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			as.Post("/echo1?aaa=11&bbb=_o_", s.Map{
				"ccc": "ccc",
				"DDD": 101.123,
				"eEe": true,
				"fff": nil,
				"ggg": 223,
			})
		}
	})
}

func BenchmarkEchosForStructNoLog(tb *testing.B) {
	tb.StopTimer()
	s.ResetAllSets()
	s.Register(0, "/echo1", Echo1)
	_ = os.Setenv("SERVICE_LOGFILE", os.DevNull)

	as := s.AsyncStart()
	defer as.Stop()

	as.Post("/echo1?aaa=11&bbb=_o_", s.Map{})

	tb.StartTimer()

	tb.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			as.Post("/echo1?aaa=11&bbb=_o_", s.Map{
				"ccc": "ccc",
				"DDD": 101.123,
				"eEe": true,
				"fff": nil,
				"ggg": 223,
			})
		}
	})
}

func BenchmarkEchosForMapNoLog(tb *testing.B) {
	tb.StopTimer()
	s.ResetAllSets()
	s.Register(0, "/echo2", Echo2)
	_ = os.Setenv("SERVICE_LOGFILE", os.DevNull)

	as := s.AsyncStart()
	defer as.Stop()

	as.Get("/echo2?aaa=11&bbb=_o_", "CID", "test-client")

	tb.StartTimer()

	for i := 0; i < tb.N; i++ {

		as.Post("/echo2?aaa=11&bbb=_o_", s.Map{
			"ccc": "ccc",
			"DDD": 101.123,
			"eEe": true,
			"fff": nil,
			"ggg": 223,
		})

	}
}
