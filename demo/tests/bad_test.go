package tests

import (
	"testing"
	"../.."
	"../userServices"
)

func init() {
	service.Register("/login", userServices.Login)
}

func TestLoginWithoutClientId(tt *testing.T) {
	t := service.T(tt)

	service.StartTestService()
	defer service.StopTestService()

	code, message, result := service.TestService("/login", nil)
	t.Test( code == 403 && message == "Not a valid client" && result.(bool) == false, "Login", message, result)
}

func TestLoginWithBadPassword(tt *testing.T) {
	t := service.T(tt)

	service.SetTestHeader("ClientId", "aabbcc")
	service.StartTestService()
	defer service.StopTestService()

	code, message, result := service.TestService("/login", map[string]interface{}{
		"account": "admin",
		"password": "xxx",
	})
	t.Test( code == 403 && message == "No Access" && result.(bool) == false, "Login", message, result)
}
