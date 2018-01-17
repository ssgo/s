package userServices

func Login(in struct {
	Account  string
	Password string
}, h struct{ ClientId string }) (out struct {
	Code int
	Ok   bool
}) {
	out.Code = 403
	if in.Account == "admin" && in.Password == "admin123" {
		out.Code = 200
		out.Ok = true
	}
	return
}
