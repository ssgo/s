package s

import (
	"regexp"
	"strings"
)

type UserAgent struct {
	Type           string
	Device         string
	BrowserName    string
	BrowserVersion string
	OSName         string
	OSVersion      string
	NetType        string
	Language       string
	AppName        string
	AppVersion     string
}

func ParseUA(userAgent string) UserAgent {
	ua := UserAgent{}
	uaInfo := parse(userAgent)
	lowerUserAgent := strings.ToLower(userAgent)

	ua.NetType = uaInfo["NetType"]
	ua.Language = uaInfo["Language"]

	// 操作系统信息
	switch {
	case uaInfo.has("Android"):
		ua.OSName = "Android"
		ua.OSVersion = uaInfo["Android"]
		for s := range uaInfo {
			if strings.HasSuffix(s, "Build") && s != "Build" {
				ua.Device = strings.TrimSpace(s[:len(s)-5])
			} else if strings.HasPrefix(s, "SAMSUNG") {
				ua.Device = s
			}
		}

	case uaInfo.has("iPhone"):
		ua.OSName = "iOS"
		ua.OSVersion = uaInfo.macVersion()
		ua.Device = "iPhone"
		ua.Type = "Mobile"

	case uaInfo.has("iPad"):
		ua.OSName = "iOS"
		ua.OSVersion = uaInfo.macVersion()
		ua.Device = "iPad"
		ua.Type = "Tablet"

	case uaInfo.has("Windows NT"):
		ua.OSName = "Windows"
		ua.OSVersion = uaInfo["Windows NT"]
		ua.Type = "Desktop"

	case uaInfo.has("Windows Phone OSName"):
		ua.OSName = "Windows Phone"
		ua.OSVersion = uaInfo["Windows Phone OSName"]
		ua.Type = "Mobile"

	case uaInfo.has("Macintosh"):
		ua.OSName = "macOS"
		ua.OSVersion = uaInfo.macVersion()
		ua.Type = "Desktop"

	case uaInfo.has("Linux"):
		ua.OSName = "Linux"
		ua.OSVersion = uaInfo["Linux"]
		ua.Type = "Desktop"
	}

	// 浏览器信息
	if strings.Contains(lowerUserAgent, "bot") || strings.Contains(lowerUserAgent, "spider") {
		ua.BrowserName, ua.BrowserVersion = uaInfo.specialName(true)
	}
	if ua.BrowserName == "" {
		switch {
		case uaInfo["Opera Mini"] != "":
			ua.BrowserName = "Opera Mini"
			ua.BrowserVersion = uaInfo["Opera Mini"]
			if ua.Type == "" {
				ua.Type = "Mobile"
			}

		case uaInfo["OPR"] != "":
			ua.BrowserName = "Opera"
			ua.BrowserVersion = uaInfo["OPR"]
			if ua.Type == "" && uaInfo.has("Mobile", "Mobile Safari") {
				ua.Type = "Mobile"
			}

		case uaInfo["OPT"] != "":
			ua.BrowserName = "Opera Touch"
			ua.BrowserVersion = uaInfo["OPT"]
			if ua.Type == "" && uaInfo.has("Mobile", "Mobile Safari") {
				ua.Type = "Mobile"
			}

		// Opera on iOS
		case uaInfo["OPiOS"] != "":
			ua.BrowserName = "Opera"
			ua.BrowserVersion = uaInfo["OPiOS"]
			if ua.Type == "" && uaInfo.has("Mobile", "Mobile Safari") {
				ua.Type = "Mobile"
			}

		// Chrome on iOS
		case uaInfo["CriOS"] != "":
			ua.BrowserName = "Chrome"
			ua.BrowserVersion = uaInfo["CriOS"]
			if ua.Type == "" && uaInfo.has("Mobile", "Mobile Safari") {
				ua.Type = "Mobile"

			}

		// Firefox on iOS
		case uaInfo["FxiOS"] != "":
			ua.BrowserName = "Firefox"
			ua.BrowserVersion = uaInfo["FxiOS"]

		case uaInfo["Firefox"] != "":
			ua.BrowserName = "Firefox"
			ua.BrowserVersion = uaInfo["Firefox"]

		case uaInfo["Vivaldi"] != "":
			ua.BrowserName = "Vivaldi"
			ua.BrowserVersion = uaInfo["Vivaldi"]

		case uaInfo.has("MSIE"):
			ua.BrowserName = "Internet Explorer"
			ua.BrowserVersion = uaInfo["MSIE"]

		case uaInfo["EdgiOS"] != "":
			ua.BrowserName = "Edge"
			ua.BrowserVersion = uaInfo["EdgiOS"]

		case uaInfo["Edge"] != "":
			ua.BrowserName = "Edge"
			ua.BrowserVersion = uaInfo["Edge"]

		case uaInfo["Edg"] != "":
			ua.BrowserName = "Edge"
			ua.BrowserVersion = uaInfo["Edg"]

		case uaInfo["EdgA"] != "":
			ua.BrowserName = "Edge"
			ua.BrowserVersion = uaInfo["EdgA"]

		case uaInfo["bingbot"] != "":
			ua.BrowserName = "Bingbot"
			ua.BrowserVersion = uaInfo["bingbot"]

		case uaInfo["SamsungBrowser"] != "":
			ua.BrowserName = "Samsung Browser"
			ua.BrowserVersion = uaInfo["SamsungBrowser"]

		case uaInfo.has("Chrome") && uaInfo.has("Safari", "Mobile Safari"):
			ua.BrowserName, ua.BrowserVersion = uaInfo.specialName(false)
			if ua.BrowserName != "" {
				break
			}
			fallthrough

		case uaInfo.has("Chrome"):
			ua.BrowserName = "Chrome"
			ua.BrowserVersion = uaInfo["Chrome"]

		case uaInfo.has("Safari", "Mobile Safari"):
			ua.BrowserName = "Safari"
			if v, ok := uaInfo["BrowserVersion"]; ok {
				ua.BrowserVersion = v
			} else {
				ua.BrowserVersion = uaInfo["Safari"]
			}
		}
	}

	// 修复一些特殊情况
	if (ua.BrowserName == "" || ua.BrowserName == "Safari") && ua.OSName == "Android" && uaInfo["BrowserVersion"] != "" {
		ua.BrowserName = "Android browser"
		ua.BrowserVersion = uaInfo["BrowserVersion"]
		ua.Type = "Mobile"
	} else {
		if ua.BrowserName == "" {
			ua.BrowserName, ua.BrowserVersion = uaInfo.specialName(true)
		}
		if ua.Type == "" && uaInfo.has("Mobile", "Mobile Safari") {
			ua.Type = "Mobile"
		}
		if ua.Type == "" && strings.Contains(strings.ToLower(ua.BrowserName), "bot") {
			ua.Type = "Bot"
		}
	}

	if ua.Type == "" && uaInfo.has("Tablet") {
		ua.Type = "Tablet"
	}
	if ua.Type == "" && uaInfo.has("Mobile", "Mobile Safari") {
		ua.Type = "Mobile"
	}

	//fmt.Println("$", ua.Type, ua.Device, ua.OSName, ua.OSVersion, ua.BrowserName, ua.BrowserVersion, ua.NetType, ua.Language)
	return ua
}

func push(m map[string]string, s string) {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "://") {
		return
	}
	//fmt.Println("#", s)
	k := s
	v := ""
	pos := strings.IndexByte(s, '/')
	if pos != -1 {
		k = s[0:pos]
		v = s[pos+1:]
	} else {
		pos = strings.LastIndexByte(s, ' ')
		if pos != -1 {
			switch s[:pos] {
			case "Linux", "Windows NT", "Windows Phone OSName", "MSIE", "Android":
				k = s[0:pos]
				v = s[pos+1:]
			}
		}
	}

	switch strings.ToLower(k) {
	case "khtml, like gecko", "u", "compatible", "mozilla", "wow64":
		return
	case "nettype", "language", "process", "channel", "chrome", "firefox", "safari", "browserversion", "mobile", "mobile safari", "applewebkit", "windows nt", "windows phone osname", "android", "macintosh", "linux", "gsa":
	default:
		if !strings.HasSuffix(k, "Build") {
			if v != "" {
				m["Special"] = k
			} else {
				m["Special2"] = k
			}
		}
	}

	m[k] = v
}

func parse(userAgent string) (clients uaInfoType) {
	clients = make(map[string]string, 0)
	i := 0
	par := false
	tag := false
	for j, c := range []byte(userAgent) {
		if c == '(' || c == ')' || (par && c == ';') || (tag && c == ' ') {
			if c == '(' {
				par = true
			}
			if c == ')' {
				par = false
			}
			tag = false
			push(clients, userAgent[i:j])
			i = j + 1
		} else if c == '/' {
			tag = true
		}
	}
	push(clients, userAgent[i:])
	return clients
}

type uaInfoType map[string]string

func (uaInfo uaInfoType) has(keys ...string) bool {
	for _, k := range keys {
		if _, ok := uaInfo[k]; ok {
			return true
		}
	}
	return false
}

var macVerRegex *regexp.Regexp

func (uaInfo uaInfoType) macVersion() string {
	if macVerRegex == nil {
		macVerRegex = regexp.MustCompile("[_\\d.]+")
	}

	ver := ""
	for k, v := range uaInfo {
		if strings.Contains(k, "OSName") {
			ver = macVerRegex.FindString(v)
			if ver == "" {
				ver = macVerRegex.FindString(k)
			}
			if ver != "" {
				ver = strings.ReplaceAll(ver, "_", ".")
				break
			}
		}
	}
	return ver
}

func (uaInfo uaInfoType) specialName(get2 bool) (string, string) {
	if uaInfo["Special"] != "" {
		return uaInfo["Special"], uaInfo[uaInfo["Special"]]
	} else if get2 && uaInfo["Special2"] != "" {
		return uaInfo["Special2"], uaInfo[uaInfo["Special2"]]
	}
	return "", ""
}
