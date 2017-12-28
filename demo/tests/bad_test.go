package tests

import (
	"testing"
	"../.."
	"../userServices"
)

func init() {
	s.Register("/login", userServices.Login)
}

func TestLoginWithoutClientId(tt *testing.T) {
	t := s.T(tt)

	s.StartTestService()
	defer s.StopTestService()

	code, message, result := s.TestService("/login", nil)
	t.Test( code == 403 && message == "Not a valid client" && result.(bool) == false, "Login", code, message, result)
}

func TestLoginWithBadPassword(tt *testing.T) {
	t := s.T(tt)

	s.SetTestHeader("ClientId", "aabbcc")
	s.StartTestService()
	defer s.StopTestService()

	code, message, result := s.TestService("/login", map[string]interface{}{
		"account": "admin",
		"password": "xxx",
	})
	t.Test( code == 403 && message == "No Access" && result.(bool) == false, "Login", code, message, result)
}
