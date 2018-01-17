package tests

import (
	"testing"
	"../.."
	"../userServices"
)

func init() {
	s.Register(0, "/login", userServices.Login)
}

func TestLoginWithoutClientId(tt *testing.T) {
	t := s.T(tt)

	s.StartTestService()
	defer s.StopTestService()

	r := s.TestService("/login", nil).(map[string]interface{})

	t.Test( r["code"].(float64) == 403 && r["ok"].(bool) == false, "Login", r)
}

func TestLoginWithBadPassword(tt *testing.T) {
	t := s.T(tt)

	s.SetTestHeader("ClientId", "aabbcc")
	s.StartTestService()
	defer s.StopTestService()

	r := s.TestService("/login", s.Map{
		"account": "admin",
		"password": "xxx",
	}).(map[string]interface{})
	t.Test( r["code"].(float64) == 403 && r["ok"].(bool) == false, "Login", r)
}
