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

var rewrites = make(map[string]*rewriteInfo)
var regexRewrites = make(map[string]*rewriteInfo)

var clientForRewrite *ClientPool

// 跳转
func Rewrite(path string, toPath string) {
	s := &rewriteInfo{toPath: toPath}
	if strings.Contains(toPath, "://") {
		if clientForRewrite == nil {
			clientForRewrite = GetClient1()
		}
	}

	if strings.ContainsRune(path, '(') {
		matcher, err := regexp.Compile("^" + path + "$")
		if err != nil {
			log.Print("Rewrite	Compile	", err)
		} else {
			s.matcher = matcher
			regexRewrites[path] = s
		}
	}
	if s.matcher == nil {
		rewrites[path] = s
	}
}

func processRewrite(request *http.Request, response *http.ResponseWriter, headers *map[string]string, startTime *time.Time) (string, bool) {
	// 获取路径
	requestPath := request.RequestURI
	var queryString string
	pos := strings.LastIndex(requestPath, "?")
	if pos != -1 {
		requestPath = requestPath[0:pos]
		queryString = requestPath[pos:]
	}

	// 查找 Rewrite
	var rewriteToPath *string
	ri := rewrites[requestPath]
	if ri != nil {
		rewriteToPath = &ri.toPath
	}
	if rewriteToPath == nil && len(regexRewrites) > 0 {
		for _, ri = range regexRewrites {
			finds := ri.matcher.FindAllStringSubmatch(request.RequestURI, 20)
			if len(finds) > 0 {
				toPath := ri.toPath
				for i, partValue := range finds[0] {
					toPath = strings.Replace(toPath, fmt.Sprintf("$%d", i), partValue, 10)
				}
				if !strings.ContainsRune(toPath, '?') && queryString != "" {
					toPath += queryString
				}
				rewriteToPath = &toPath
				break
			}
		}
	}

	// 处理 Rewrite
	if rewriteToPath != nil {
		log.Printf("REWRITE	%s	%s	%s	%s	%s", getRealIp(request), request.Host, request.Method, request.RequestURI, *rewriteToPath)
		if strings.Contains(*rewriteToPath, "://") {
			// 转发到外部地址
			var bodyBytes []byte = nil
			if request.Body != nil {
				bodyBytes, _ = ioutil.ReadAll(request.Body)
				request.Body.Close()
			}
			r := clientForRewrite.Do(request.Method, *rewriteToPath, bodyBytes)
			outBytes := r.Bytes()
			(*response).Write(outBytes)
			if recordLogs {
				writeLog("REWRITE", outBytes, false, request, response, nil, headers, startTime, 0, 200)
			}
			return "", true
		} else {
			// 直接修改内部跳转地址
			requestPath = *rewriteToPath
		}
	}

	return requestPath, false
}
