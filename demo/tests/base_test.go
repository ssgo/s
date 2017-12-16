package tests

import (
	"testing"
	"../.."
	"../userServices"
	"strings"
)

func init() {
	service.Register("/", userServices.Index)
	service.Register("/login", userServices.Login)
}

func TestIndex(tt *testing.T) {
	t := service.T(tt)

	service.StartTestService()
	defer service.StopTestService()

	_, result, err := service.TestGet("/")
	t.Test( strings.Contains(string(result), "Hello World!"), "Index", string(result), err)
}


func TestLoginOK(tt *testing.T) {
	t := service.T(tt)

	service.SetTestHeader("ClientId", "aabbcc")
	service.StartTestService()
	defer service.StopTestService()

	code, _, result := service.TestService("/login", map[string]interface{}{
		"account": "admin",
		"password": "admin123",
	})
	t.Test( code == 200 && result.(bool) == true, "Login", result)
}
