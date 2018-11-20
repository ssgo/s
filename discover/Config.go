package discover

var config Config

type Config struct {
	Registry             string               // "127.0.0.1:6379:15"、"127.0.0.1:6379:15:password"
	RegistryAllowTimeout string               // "127.0.0.1:6379:15:0"、"127.0.0.1:6379:15:password:-1"
	RegistryPrefix       string               // ""
	RegistryCalls        string               // same with RegistryHost
	App                  string               // register to a app service
	Weight               uint                 // 1
	Calls                map[string]*CallInfo // defines which apps will call
	CallRetryTimes       uint8                // 10
	XUniqueId            string               // X-Unique-Id
	XForwardedForName    string               // X-Forwarded-For
	XRealIpName          string               // X-Real-Ip
	CallTimeout          int                  // 10000
}

type CallInfo struct {
	Headers     map[string]string
	Timeout     int
	HttpVersion int
	WithSSL     bool
}
