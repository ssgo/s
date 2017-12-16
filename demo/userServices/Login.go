package userServices

func Login(in struct {
	Account   string
	Password  string
	ClientId  string
}) (int, string, bool) {
	if in.ClientId == ""{
		return 403, "Not a valid client", false
	}
	if in.Account == "admin" && in.Password == "admin123" {
		return 200, "Login OK", true
	}

	return 403, "No Access", false
}
