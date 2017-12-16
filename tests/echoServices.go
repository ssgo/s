package tests

import (
	"log"
	"net/http"
)

type echo1Args struct {
	Aaa int
	Bbb string
	Ccc string
	Ddd float32
	Eee bool
	Fff interface{}
	Ggg string
}

func Echo1(in echo1Args) (code int, message string, data interface{}) {
	return 211, "OK", in
}

func Echo2(in map[string]interface{}) (code int, message string, data interface{}) {
	delete(in, "RedisPool")
	delete(in, "HttpRequest")
	delete(in, "HttpResponse")
	return 211, "OK", in
}

func Echo3(in struct {
	Name      string
	RedisPool string
	HttpRequestPath string
	HttpRequest *http.Request
}) (code int, message string, data interface{}) {
	log.Println(in)
	return 211, "OK", []interface{}{in.Name, in.RedisPool, in.HttpRequestPath, in.HttpRequest.RequestURI}
}
