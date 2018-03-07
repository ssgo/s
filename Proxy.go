package s

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type proxyInfo struct {
	matcher *regexp.Regexp
	toApp   string
	toPath  string
}

var proxies = make(map[string]*proxyInfo, 0)
var regexProxies = make(map[string]*proxyInfo, 0)
var proxyBy func(*http.Request) (*string, *string, *map[string]string)

// 跳转
func SetProxyBy(by func(request *http.Request) (toApp, toPath *string, headers *map[string]string)) {
	//forceDiscoverClient = true // 代理模式强制启动 Discover Client
	proxyBy = by
}

// 代理
func Proxy(path string, toApp, toPath string) {
	p := &proxyInfo{toApp: toApp, toPath: toPath}
	if strings.Contains(path, "(") {
		matcher, err := regexp.Compile("^" + path + "$")
		if err != nil {
			log.Print("Proxy Error	Compile	", err)
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
func findProxy(request *http.Request) (*string, *string) {
	var requestPath string
	var queryString string
	pos := strings.LastIndex(request.RequestURI, "?")
	if pos != -1 {
		requestPath = request.RequestURI[0:pos]
		queryString = requestPath[pos:]
	} else {
		requestPath = request.RequestURI
	}
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
				if queryString != "" {
					toPath += queryString
				}
				return &pi.toApp, &toPath
			}
		}
	}
	return nil, nil
}

// ProxyBy
func processProxy(request *http.Request, response *http.ResponseWriter, headers *map[string]string, startTime *time.Time) (finished bool) {
	proxyToApp, proxyToPath := findProxy(request)
	var proxyHeaders *map[string]string
	if proxyBy != nil && (proxyToApp == nil || proxyToPath == nil || *proxyToApp == "" || *proxyToPath == "") {
		proxyToApp, proxyToPath, proxyHeaders = proxyBy(request)
	}
	if proxyToApp == nil || proxyToPath == nil || *proxyToApp == "" || *proxyToPath == "" {
		return false
	}

	if recordLogs {
		log.Printf("PROXY	%s	%s	%s	%s	%s	%s", getRealIp(request), request.Host, request.Method, request.RequestURI, *proxyToApp, *proxyToPath)
	}

	// 注册新的Call，并重启订阅
	if appClientPools[*proxyToApp] == nil {
		AddCall(*proxyToApp, Call{})
		RestartDiscoverSyncer()
	}

	// 处理 Proxy
	var bodyBytes []byte = nil
	if request.Body != nil {
		bodyBytes, _ = ioutil.ReadAll(request.Body)
		request.Body.Close()
	}
	caller := &Caller{request: request}
	requestHeaders := make([]string, 0)
	if proxyHeaders != nil {
		for k, v := range *proxyHeaders {
			requestHeaders = append(requestHeaders, k, v)
		}
	}
	requestHeaders = append(requestHeaders, "Host", request.Host)
	r := caller.Do(request.Method, *proxyToApp, *proxyToPath, bodyBytes, requestHeaders...)

	var statusCode int
	var outBytes []byte
	if r.Error == nil && r.Response != nil {
		statusCode = r.Response.StatusCode
		outBytes = r.Bytes()
		for k, v := range r.Response.Header {
			(*response).Header().Set(k, v[0])
		}
	} else {
		statusCode = 500
		outBytes = []byte(r.Error.Error())
	}

	(*response).WriteHeader(statusCode)
	(*response).Write(outBytes)
	if recordLogs {
		outLen := 0
		if outBytes != nil {
			outLen = len(outBytes)
		}
		outBytes = nil
		writeLog("REDIRECT", outBytes, outLen, false, request, response, nil, headers, startTime, 0, statusCode)
	}
	return true

	//var statusCode int
	//caller := &Caller{request: request}
	//requestHeaders := make([]string, 0)
	//if proxyHeaders != nil {
	//	for k, v := range *proxyHeaders {
	//		requestHeaders = append(requestHeaders, k, v)
	//	}
	//}
	//requestHeaders = append(requestHeaders, "Host", request.Host)
	//r := caller.Do(request.Method, *proxyToApp, *proxyToPath, args, requestHeaders...)
	//result := r.Bytes()
	//statusCode = 500
	//if r.Error == nil && r.Response != nil {
	//	statusCode = r.Response.StatusCode
	//}

}
