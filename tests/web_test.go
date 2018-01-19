package tests

import (
	"testing"
	".."
	"net/http"
	"os"
)

func Welcome(in struct{}) string {
	return "Hello World!"
}

func WelcomePicture(in struct{ PicName string }, response *http.ResponseWriter) []byte {
	(*response).Header().Set("Content-Type", "image/png")
	pic := make([]byte, 5)
	bytePicName := []byte(in.PicName)
	pic[0] = 1
	pic[1] = 0
	pic[2] = 240
	pic[3] = bytePicName[0]
	pic[4] = bytePicName[1]
	return pic
}

func TestWelcomeWithHttp1(tt *testing.T) {
	t := s.T(tt)

	s.ResetAllSets()
	s.Register(0, "/", Welcome)
	as := s.AsyncStart1()

	r := as.Get("/")
	t.Test(r.Error == nil && r.String() == "Hello World!", "Welcome", r.Error, r.String())
	t.Test(r.Response.Proto == "HTTP/1.1", "Welcome HTTP/1.1", r.Error, r.Response.Proto)

	as.Stop()
}

func TestWelcomeWithHttp2(tt *testing.T) {
	t := s.T(tt)

	s.ResetAllSets()
	s.Register(0, "/", Welcome)
	as := s.AsyncStart()

	c := s.GetClient()
	r := c.Do("http://"+as.Addr, nil)
	t.Test(r.Error == nil && r.String() == "Hello World!", "Welcome", r.Error, r.String())
	t.Test(r.Response.Proto == "HTTP/2.0", "Welcome Proto", r.Error, r.Response.Proto)

	as.Stop()
}

func TestWelcomePicture(tt *testing.T) {
	t := s.T(tt)

	s.ResetAllSets()
	s.Register(0, "/w/{picName}.png", WelcomePicture)
	os.Setenv("SERVICE_LOGFILE", os.DevNull)

	as := s.AsyncStart()
	defer as.Stop()

	r := as.Get("/w/abc.png")
	result := r.Bytes()
	t.Test(r.Error == nil && result[0] == 1 && result[1] == 0 && result[2] == 240 && result[4] == 'b', "WelcomePicture", result, r.Error)
	t.Test(r.Response.Header.Get("Content-Type") == "image/png", "WelcomePicture Content-Type", result, r.Error)
}

