package tests

import (
	"testing"
	".."
	"net/http"
)

func Welcome(in map[string]interface{}) string {
	return "Hello World!"
}

func WelcomePicture(in struct{
	HttpResponse http.ResponseWriter
}) []byte {
	in.HttpResponse.Header().Set("Content-Type", "image/png")
	pic := make([]byte, 3)
	pic[0] = 1
	pic[1] = 0
	pic[2] = 240
	return pic
}

func TestWelcome(tt *testing.T) {
	t := service.T(tt)

	service.ResetAllSets()
	service.Register("/", Welcome)

	service.StartTestService()
	defer service.StopTestService()

	_, result, err := service.TestGet("/")
	t.Test(err == nil && string(result) == "Hello World!", "Welcome", string(result), err)
}

func TestWelcomePicture(tt *testing.T) {
	t := service.T(tt)

	service.ResetAllSets()
	service.RegisterByRegex("/w/.+?\\.png", WelcomePicture)

	service.StartTestService()
	defer service.StopTestService()

	res, result, err := service.TestGet("/w/abc.png")
	t.Test(err == nil && result[0] == 1 && result[1] == 0 && result[2] == 240, "WelcomePicture", result, err)
	t.Test(res.Header.Get("Content-Type") == "image/png", "WelcomePicture Content-Type", result, err)
}
