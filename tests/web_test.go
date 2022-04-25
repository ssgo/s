package tests

import (
	"fmt"
	"github.com/ssgo/httpclient"
	"net/http"
	"os"
	"testing"

	"github.com/ssgo/s"
)

func Welcome() string {
	return "Hello World!"
}

func WelcomePicture(in struct{ PicName string }, response http.ResponseWriter) []byte {
	response.Header().Set("Content-Type", "image/png")
	pic := make([]byte, 5)
	bytePicName := []byte(in.PicName)
	pic[0] = 1
	pic[1] = 0
	pic[2] = 240
	pic[3] = bytePicName[0]
	pic[4] = bytePicName[1]
	return pic
}

//func TestStatic(tt *testing.T) {
//	t := s.T(tt)
//	s.ResetAllSets()
//	s.Static("/", "www")
//
//	as := s.AsyncStart1()
//	r := as.Get("/")
//	t.Test(r.Error == nil && strings.Contains(r.String(), "Hello"), "Static /", r.Error, r.String())
//	r = as.Get("/aaa/111.json")
//	t.Test(r.Error == nil && r.String() == "111", "Static 111.json", r.Error, r.String())
//	r = as.Get("/ooo.html")
//	t.Test(r.Error == nil && r.Response.StatusCode == 404, "Static 404", r.Error, r.String())
//	as.Stop()
//}

func TestWelcomeWithRestful(tt *testing.T) {
	t := s.T(tt)

	//_ = os.Setenv("service_httpVersion", "1")
	_ = os.Setenv("service_listen", ":,http")
	_ = os.Setenv("service_fast", "true")
	s.ResetAllSets()
	s.Restful(0, "GET", "/", Welcome)
	s.Restful(0, "PULL", "/w/{picName}.png", WelcomePicture)
	fmt.Println("000")
	as := s.AsyncStart()

	r := as.Get("/")
	t.Test(r.Error == nil && r.String() == "Hello World!", "Get", r.Error, r.String())

	r = as.Post("/", nil)
	t.Test(r.Response.StatusCode == 404, "Post", r.Response.StatusCode, r.Error, r.String())

	r = as.Get("/w/abc.png")
	t.Test(r.Response.StatusCode == 404, "Post", r.Response.StatusCode, r.Error, r.String())

	r = as.Do("PULL", "/w/abc.png", nil)
	result := r.Bytes()
	t.Test(r.Error == nil && result[0] == 1 && result[1] == 0 && result[2] == 240 && result[4] == 'b', "WelcomePicture", result, r.Error)
	t.Test(r.Response.Header.Get("Content-Type") == "image/png", "WelcomePicture Content-Type", result, r.Error)
	fmt.Println("111")
	as.Stop()
	fmt.Println("222")
}

func TestWelcomeWithHttp1(tt *testing.T) {
	t := s.T(tt)

	//_ = os.Setenv("service_httpVersion", "1")
	_ = os.Setenv("service_listen", ":,http")
	s.ResetAllSets()
	s.Register(0, "/", Welcome)
	as := s.AsyncStart()

	r := as.Get("/")
	t.Test(r.Error == nil && r.String() == "Hello World!", "Welcome", r.Error, r.String())
	t.Test(r.Response.Proto == "HTTP/1.1", "Welcome HTTP/1.1", r.Error, r.Response.Proto)

	as.Stop()
}

func TestWelcomeWithHttp2(tt *testing.T) {
	t := s.T(tt)

	//_ = os.Unsetenv("service_httpVersion")
	_ = os.Setenv("service_listen", ":,h2c")
	s.ResetAllSets()
	s.Register(0, "/", Welcome)
	as := s.AsyncStart()

	c := httpclient.GetClientH2C(1000)
	r := c.Get("http://" + as.Addr)
	t.Test(r.Error == nil && r.String() == "Hello World!", "Welcome", r.Error, r.String())
	t.Test(r.Response.Proto == "HTTP/2.0", "Welcome Proto", r.Error, r.Response.Proto)

	as.Stop()
}

func TestWelcomePicture(tt *testing.T) {
	t := s.T(tt)

	_ = os.Setenv("LOG_FILE", os.DevNull)
	s.ResetAllSets()
	s.Register(0, "/w/{picName}.png", WelcomePicture)

	as := s.AsyncStart()
	defer as.Stop()

	r := as.Get("/w/abc.png")
	result := r.Bytes()
	t.Test(r.Error == nil && result[0] == 1 && result[1] == 0 && result[2] == 240 && result[4] == 'b', "WelcomePicture", result, r.Error)
	t.Test(r.Response.Header.Get("Content-Type") == "image/png", "WelcomePicture Content-Type", result, r.Error)
}
