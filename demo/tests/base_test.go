package tests

import (
	"testing"
	"../.."
	"../userServices"
	"strings"
)

func init() {
	s.Register("/", userServices.Index)
	s.Register("/login", userServices.Login)
}

func TestIndex(tt *testing.T) {
	t := s.T(tt)

	s.StartTestService()
	defer s.StopTestService()

	_, result, err := s.TestGet("/")
	t.Test( strings.Contains(string(result), "Hello World!"), "Index", string(result), err)
}


func TestLoginOK(tt *testing.T) {
	t := s.T(tt)

	s.SetTestHeader("ClientId", "aabbcc")
	s.StartTestService()
	defer s.StopTestService()

	code, _, result := s.TestService("/login", map[string]interface{}{
		"account": "admin",
		"password": "admin123",
	})
	t.Test( code == 200 && result.(bool) == true, "Login", result)
}
