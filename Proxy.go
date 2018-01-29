package s

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

var proxies = make(map[string]*proxyInfo, 0)
var regexProxies = make(map[string]*proxyInfo, 0)

// 代理
func Proxy(authLevel uint, path string, toApp, toPath string) {
	p := &proxyInfo{authLevel: authLevel, toApp: toApp, toPath: toPath}
	if strings.Contains(path, "(") {
		matcher, err := regexp.Compile("^" + path + "$")
		if err != nil {
			log.Print("Proxy	Compile	", err)
		} else {
			p.matcher = matcher
			regexProxies[path] = p
		}
	}
	if p.matcher == nil {
		proxies[path] = p
	}
}

// 查找 Proxy
func findProxy(requestPath string) (*string, *string) {
	pi := proxies[requestPath]
	if pi != nil {
		return &pi.toApp, &pi.toPath
	}
	if len(regexProxies) > 0 {
		for _, pi := range regexProxies {
			finds := pi.matcher.FindAllStringSubmatch(requestPath, 20)
			if len(finds) > 0 {
				toPath := pi.toPath
				for i, partValue := range finds[0] {
					toPath = strings.Replace(toPath, fmt.Sprintf("$%d", i), partValue, 10)
				}
				return &pi.toApp, &toPath
			}
		}
	}
	return nil, nil
}
