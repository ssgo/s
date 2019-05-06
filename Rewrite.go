package s

import (
	"fmt"
	"github.com/ssgo/log"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ssgo/httpclient"
	"github.com/ssgo/u"
)

type rewriteInfo struct {
	matcher     *regexp.Regexp
	httpVersion int
	fromPath    string
	toPath      string
}

var rewrites = make(map[string]*rewriteInfo)
var rewriteBy func(*http.Request) (string, int, *map[string]string, bool)
var regexRewrites = make([]*rewriteInfo, 0)

var clientForRewrite1 *httpclient.ClientPool
var clientForRewrite2 *httpclient.ClientPool

// 跳转
func setRewrite(path string, toPath string, httpVersion int) {
	s := &rewriteInfo{fromPath: path, toPath: toPath, httpVersion: httpVersion}

	if strings.ContainsRune(path, '(') {
		matcher, err := regexp.Compile("^" + path + "$")
		if err != nil {
			logError(err.Error(), Map{
				"fromPath":    path,
				"toPath":      toPath,
				"httpVersion": httpVersion,
			})
			//log.Print("Rewrite Error	Compile	", err)
		} else {
			s.matcher = matcher
			regexRewrites = append(regexRewrites, s)
		}
	}
	if s.matcher == nil {
		rewrites[path] = s
	}
}
func Rewrite(path string, toPath string) {
	setRewrite(path, toPath, 2)
}
func Rewrite1(path string, toPath string) {
	setRewrite(path, toPath, 1)
}

// 跳转
func SetRewriteBy(by func(request *http.Request) (toPath string, httpVersion int, headers *map[string]string, rewrite bool)) {
	rewriteBy = by
}

func processRewrite(request *http.Request, response *Response, headers *map[string]string, startTime *time.Time, requestLogger *log.Logger) (finished bool) {
	// 获取路径
	requestPath := request.RequestURI
	var queryString string
	pos := strings.LastIndex(requestPath, "?")
	if pos != -1 {
		queryString = requestPath[pos:]
		requestPath = requestPath[0:pos]
	}
	// 查找 Rewrite
	var rewriteToPath *string
	var rewriteHttpVersion int
	var rewriteHeaders *map[string]string
	ri := rewrites[requestPath]
	if ri != nil {
		rewriteToPath = &ri.toPath
		rewriteHttpVersion = ri.httpVersion
	}
	if rewriteToPath == nil && rewriteBy != nil {
		rp, hv, h, rewrite := rewriteBy(request)
		if rewrite {
			rewriteToPath = &rp
			rewriteHttpVersion = hv
			rewriteHeaders = h
		}
	}
	if rewriteToPath == nil && len(regexRewrites) > 0 {
		for _, ri = range regexRewrites {
			finds := ri.matcher.FindAllStringSubmatch(request.RequestURI, 20)
			if len(finds) > 0 {
				toPath := ri.toPath
				for i, partValue := range finds[0] {
					toPath = strings.Replace(toPath, fmt.Sprintf("$%d", i), partValue, 10)
				}
				rewriteToPath = &toPath
				rewriteHttpVersion = ri.httpVersion
				break
			}
		}
	}

	// 处理 Rewrite
	if rewriteToPath != nil {
		if strings.Contains(*rewriteToPath, "://") {
			if !strings.ContainsRune(*rewriteToPath, '?') && queryString != "" {
				*rewriteToPath += queryString
			}
			//if recordLogs {
			//	log.Printf("REWRITE	%s	%s	%s	%s	%s", getRealIp(request), request.Host, request.Method, request.RequestURI, *rewriteToPath)
			//}

			// 转发到外部地址
			//var bodyBytes []byte = nil
			//if request.Body != nil {
			//	bodyBytes, _ = ioutil.ReadAll(request.Body)
			//	request.Body.Close()
			//}
			if rewriteHttpVersion == 1 && clientForRewrite1 == nil {
				clientForRewrite1 = httpclient.GetClient(time.Duration(Config.RewriteTimeout) * time.Millisecond)
				clientForRewrite1.NoBody = true
			}
			if rewriteHttpVersion == 2 && clientForRewrite2 == nil {
				clientForRewrite2 = httpclient.GetClientH2C(time.Duration(Config.RewriteTimeout) * time.Millisecond)
				clientForRewrite2.NoBody = true
			}
			requestHeaders := make([]string, 0)
			if rewriteHeaders != nil {
				for k, v := range *rewriteHeaders {
					requestHeaders = append(requestHeaders, k, v)
				}
			}
			c := u.If(rewriteHttpVersion == 2, clientForRewrite2, clientForRewrite1).(*httpclient.ClientPool)
			r := c.DoByRequest(request, request.Method, *rewriteToPath, request.Body, requestHeaders...)

			//var statusCode int
			//outLen := 0
			//var outBytes []byte
			if r.Error == nil && r.Response != nil {
				//statusCode = r.Response.StatusCode
				//outBytes = r.Bytes()
				for k, v := range r.Response.Header {
					response.Header().Set(k, v[0])
				}
				response.WriteHeader(r.Response.StatusCode)
				//copiedLen, err := io.Copy(response.writer, r.Response.Body)
				copiedLen, err := io.Copy(response.writer, r.Response.Body)
				if err != nil {
					response.WriteHeader(500)
					n, err := response.Write([]byte(err.Error()))
					if err != nil {
						requestLogger.Error(err.Error(), "wrote", n)
					}
					response.outLen = int(len(err.Error()))
				} else {
					response.outLen = int(copiedLen)
				}

			} else {
				//statusCode = 500
				//outBytes = []byte(r.Error.Error())
				response.WriteHeader(500)
				n, err := response.Write([]byte(r.Error.Error()))
				if err != nil {
					requestLogger.Error(err.Error(), "wrote", n)
				}
				response.outLen = int(len(r.Error.Error()))
			}

			writeLog(requestLogger, "REWRITE", nil, response.outLen, request, response, nil, headers, startTime, 0, Map{
				"toPath":         rewriteToPath,
				"rewriteHeaders": rewriteHeaders,
				"httpVersion":    rewriteHttpVersion,
			})
			return true
		} else {
			// 直接修改内部跳转地址
			requestLogger.Info("rewrite", Map{
				"fromPath":       request.RequestURI,
				"toPath":         rewriteToPath,
				"httpVersion":    rewriteHttpVersion,
				"rewriteHeaders": rewriteHeaders,
				"ip":             getRealIp(request),
				"method":         request.Method,
				"host":           request.Host,
			})

			request.RequestURI = *rewriteToPath
			if queryString != "" && !strings.Contains(request.RequestURI, "?") {
				request.RequestURI += queryString
			}
		}
	}
	return false
}
