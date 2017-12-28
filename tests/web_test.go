package tests

import (
	"testing"
	".."
	"net/http"
)

func Welcome(in struct{}) string {
	return "Hello World!"
}

func WelcomePicture(in struct{PicName string}, response http.ResponseWriter) []byte {
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

func TestWelcome(tt *testing.T) {
	t := s.T(tt)

	s.ResetAllSets()
	s.Register("/", Welcome)
	s.EnableLogs(false)

	s.StartTestService()
	defer s.StopTestService()

	_, result, err := s.TestGet("/")
	t.Test(err == nil && string(result) == "Hello World!", "Welcome", string(result), err)
}

func TestWelcomePicture(tt *testing.T) {
	t := s.T(tt)

	s.ResetAllSets()
	s.Register("/w/{picName}.png", WelcomePicture)
	s.EnableLogs(false)

	s.StartTestService()
	defer s.StopTestService()

	res, result, err := s.TestGet("/w/abc.png")
	t.Test(err == nil && result[0] == 1 && result[1] == 0 && result[2] == 240 && result[4] == 'b', "WelcomePicture", result, err)
	t.Test(res.Header.Get("Content-Type") == "image/png", "WelcomePicture Content-Type", result, err)
}
