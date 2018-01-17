package tests

import (
	"testing"
	"../.."
	"../userServices"
	"strings"
)

func init() {
	s.Register(0, "/", userServices.Index)
	s.Register(0, "/login", userServices.Login)
}

func TestIndex(tt *testing.T) {
	t := s.T(tt)

	s.StartTestService()
	defer s.StopTestService()

	_, result, err := s.TestGet("/")
	t.Test( strings.Contains(string(result), "Hello World!"), "Index", string(result), err)
}


func T1estLoginOK(tt *testing.T) {
	t := s.T(tt)

	s.SetTestHeader("ClientId", "aabbcc")
	s.StartTestService()
	defer s.StopTestService()

	r := s.TestService("/login", s.Map{
		"account": "admin",
		"password": "admin123",
	}).(map[string]interface{})
	t.Test( r["code"].(float64) == 200 && r["ok"].(bool) == true, "Login", r)
}
