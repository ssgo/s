package tests

import (
	"testing"
	".."
	"fmt"
)

func TestEchos(tt *testing.T) {
	t := s.T(tt)

	//s.SetContext("RedisPool", "My name is RedisPool")
	s.Register("/echo1", Echo1)
	s.Register("/echo2", Echo2)
	s.Register("/echo3", Echo3)
	s.SetTestHeader("Cid", "test-client")

	s.StartTestService()
	defer s.StopTestService()
	s.EnableLogs(false)

	code, message, data := s.TestService("/echo1?aaa=11&bbb=_o_", s.Map{
		"ccc": "ccc",
		"DDD": 101.123,
		"eEe": true,
		"fff": nil,
		"ggg": "223",
	})

	t.Test(code == 211 && message == "OK", "[Echo1] Response", code, message, data)
	datas, ok := data.([]interface{})
	d1, ok := datas[0].(map[string]interface{})
	d2, ok := datas[1].(map[string]interface{})
	t.Test(ok, "[Echo1] Data1", code, message, data)
	t.Test(d1["aaa"].(float64) == 11 && d1["bbb"] == "_o_" && d1["ddd"] == 101.123 && d1["eee"] == true && d1["fff"] == nil, "[Echo1] Data2", code, message, data)
	t.Test(d2["cid"] == "test-client", "[Echo1] Data3", code, message, data)

	code, message, data = s.TestService("/echo2?aaa=11&bbb=_o_", s.Map{
		"ccc": "ccc",
		"DDD": 101.123,
		"eEe": true,
		"fff": nil,
		"ggg": 223,
	})

	t.Test(code == 211 && message == "OK", "[Echo2] Response", code, message, data)
	d, ok := data.(map[string]interface{})
	t.Test(ok, "[Echo2] Data1", code, message, data)
	t.Test(d["aaa"].(float64) == 11 && d["bbb"] == "_o_" && d["ddd"] == 101.123 && d["eee"] == true && d["fff"] == nil, "[Echo2] Data2", code, message, data)

	code, message, data = s.TestService("/echo3?a=1", s.Map{"name": "Star"})
	t.Test(code == 211, "[Echo3] Response", code, message, data)
	a, ok := data.([]interface{})
	t.Test(ok, "[Echo3] Data1", code, message, data)
	t.Test(a[0] == "Star", "[Echo3] Data2", code, message, data)
	t.Test(a[1] == "/echo3?a=1", "[Echo3] Data3", code, message, data)
}

func TestFilters(tt *testing.T) {
	t := s.T(tt)
	s.Register("/echo2", Echo2)

	s.StartTestService()
	defer s.StopTestService()
	s.EnableLogs(false)

	code, message, data := s.TestService("/echo2?aaa=11&bbb=_o_", s.Map{"ccc": "ccc"})
	d, _ := data.(map[string]interface{})
	t.Test(code == 211 && d["filterTag"] == "", "[Test InFilter 1] Response", code, message, data)

	s.SetInFilter(func(in s.Map) *s.Result {
		in["filterTag"] = "Abc"
		in["filterTag2"] = 1000
		return nil
	})
	code, message, data = s.TestService("/echo2?aaa=11&bbb=_o_", s.Map{"ccc": "ccc"})
	d, _ = data.(map[string]interface{})
	t.Test(code == 211 && d["filterTag"] == "Abc" && d["filterTag2"].(float64) == 1000, "[Test InFilter 2] Response", code, message, data)

	s.SetOutFilter(func(in s.Map, result *s.Result) *s.Result {
		result.Code ++
		data := result.Data.(map[string]interface{})
		data["filterTag2"] = data["filterTag2"].(float64) + 100
		return nil
	})

	code, message, data = s.TestService("/echo2?aaa=11&bbb=_o_", s.Map{"ccc": "ccc"})
	d, _ = data.(map[string]interface{})
	t.Test(code == 212 && d["filterTag"] == "Abc" && d["filterTag2"].(float64) == 1100, "[Test OutFilters 1] Response", code, message, data)

	s.SetOutFilter(func(in s.Map, result *s.Result) *s.Result {
		result.Code *= 2
		data := result.Data.(map[string]interface{})
		return &s.Result{Code: result.Code, Message: result.Message, Data: s.Map{
			"filterTag":  in["filterTag"],
			"filterTag2": data["filterTag2"].(float64) + 100,
		}}
	})
	code, message, data = s.TestService("/echo2?aaa=11&bbb=_o_", s.Map{"ccc": "ccc"})
	d, _ = data.(map[string]interface{})
	t.Test(code == 424 && d["filterTag"] == "Abc" && d["filterTag2"].(float64) == 1200, "[Test OutFilters 2] Response", code, message, data)

	s.SetInFilter(func(in s.Map) (*s.Result) {
		return &s.Result{Code: 212, Message: "OK", Data: s.Map{
			"filterTag":  in["filterTag"],
			"filterTag2": in["filterTag2"].(int) + 100,
		}}
	})
	code, message, data = s.TestService("/echo2?aaa=11&bbb=_o_", s.Map{"ccc": "ccc"})
	d, _ = data.(map[string]interface{})
	t.Test(code == 426 && d["filterTag"] == "Abc" && d["filterTag2"].(float64) == 1300, "[Test InFilter 3] Response", code, message, data)
}

func TestEchos2(tt *testing.T) {
	s.Register("/echo1", Echo1)
	s.Register("/echo2", Echo2)
	s.EnableLogs(false)

	s.StartTestService()
	defer s.StopTestService()

	fmt.Println()
	for i := 0; i < 5; i++ {
		s.TestService("/echo1?aaa=11&bbb=_o_", s.Map{
			"ccc": "ccc",
			"DDD": 101.123,
			"eEe": true,
			"fff": nil,
			"ggg": 223,
		})
	}

	for i := 0; i < 5; i++ {
		s.TestService("/echo2?aaa=11&bbb=_o_", s.Map{
			"ccc": "ccc",
			"DDD": 101.123,
			"eEe": true,
			"fff": nil,
			"ggg": 223,
		})
	}
}

func BenchmarkEchosForStruct(tb *testing.B) {
	tb.StopTimer()
	s.Register("/echo1", Echo1)
	s.EnableLogs(false)

	s.StartTestService()
	defer s.StopTestService()

	s.TestService("/echo1?aaa=11&bbb=_o_", s.Map{})

	tb.StartTimer()

	tb.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.TestService("/echo1?aaa=11&bbb=_o_", s.Map{
				"ccc": "ccc",
				"DDD": 101.123,
				"eEe": true,
				"fff": nil,
				"ggg": 223,
			})
		}
	})
}
//
//func BenchmarkEchosForMap(tb *testing.B) {
//	tb.StopTimer()
//	s.Register("/echo2", Echo2)
//	s.EnableLogs(false)
//	s.SetTestHeader("Cid", "test-client")
//
//	s.StartTestService()
//	defer s.StopTestService()
//
//	s.TestService("/echo2?aaa=11&bbb=_o_", s.Map{})
//
//	tb.StartTimer()
//
//	for i := 0; i < tb.N; i++ {
//
//		s.TestService("/echo2?aaa=11&bbb=_o_", s.Map{
//			"ccc": "ccc",
//			"DDD": 101.123,
//			"eEe": true,
//			"fff": nil,
//			"ggg": 223,
//		})
//
//	}
//}
