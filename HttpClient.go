package httpclient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/ssgo/base"
	"golang.org/x/net/http2"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type ClientPool struct {
	pool              *http.Client
	GlobalHeaders     map[string]string
	XUniqueId       string
	XRealIpName       string
	XForwardedForName string
}

type Result struct {
	Error    error
	Response *http.Response
	data     []byte
}

func GetClientH2C(timeout time.Duration) *ClientPool {
	if timeout < time.Millisecond {
		timeout *= time.Millisecond
	}
	clientConfig := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: timeout,
	}
	return &ClientPool{pool: clientConfig, GlobalHeaders: map[string]string{}}
}
func GetClient(timeout time.Duration) *ClientPool {
	if timeout < time.Millisecond {
		timeout *= time.Millisecond
	}
	return &ClientPool{pool: &http.Client{Timeout: timeout}, GlobalHeaders: map[string]string{}}
}

func (cp *ClientPool) EnableRedirect() {
	cp.pool.CheckRedirect = nil
}
func (cp *ClientPool) SetGlobalHeader(k, v string) {
	if v == "" {
		delete(cp.GlobalHeaders, k)
	} else {
		cp.GlobalHeaders[k] = v
	}
}

func (cp *ClientPool) Get(url string, headers ...string) *Result {
	return cp.Do("GET", url, nil, headers...)
}
func (cp *ClientPool) Post(url string, data interface{}, headers ...string) *Result {
	return cp.Do("POST", url, data, headers...)
}
func (cp *ClientPool) Put(url string, data interface{}, headers ...string) *Result {
	return cp.Do("PUT", url, data, headers...)
}
func (cp *ClientPool) Delete(url string, data interface{}, headers ...string) *Result {
	return cp.Do("DELETE", url, data, headers...)
}
func (cp *ClientPool) Head(url string, data interface{}, headers ...string) *Result {
	return cp.Do("HEAD", url, data, headers...)
}
func (cp *ClientPool) DoByRequest(request *http.Request, method, url string, data interface{}, settedHeaders ...string) *Result {
	headers := make([]string, 0)
	for k, v := range request.Header {
		headers = append(headers, k, v[0])
	}
	if cp.XForwardedForName == "" {
		cp.XForwardedForName = "X-Forwarded-For"
	}
	if cp.XUniqueId == "" {
		cp.XUniqueId = "X-Unique-Id"
	}
	if cp.XRealIpName == "" {
		cp.XRealIpName = "X-Real-Ip"
	}

	uniqueId := request.Header.Get(cp.XUniqueId)
	if request.Header.Get(cp.XUniqueId) != "" {
		headers = append(headers, cp.XUniqueId, uniqueId)
	}
	headers = append(headers, cp.XRealIpName, cp.getRealIp(request))
	headers = append(headers, cp.XForwardedForName, request.Header.Get(cp.XForwardedForName)+base.StringIf(request.Header.Get(cp.XForwardedForName) == "", "", ", ")+request.RemoteAddr[0:strings.IndexByte(request.RemoteAddr, ':')])
	headers = append(headers, settedHeaders...)
	return cp.Do(method, url, data, headers...)
}
func (cp *ClientPool) Do(method, url string, data interface{}, headers ...string) *Result {
	var req *http.Request
	var err error
	if data == nil {
		req, err = http.NewRequest(method, url, nil)
	} else {
		var bytesData []byte
		err = nil
		isJson := false
		switch t := data.(type) {
		case []byte:
			bytesData = t
		case string:
			bytesData = []byte(t)
		default:
			bytesData, err = json.Marshal(data)
			isJson = true
		}
		if err == nil {
			req, err = http.NewRequest(method, url, bytes.NewReader(bytesData))
			if isJson {
				req.Header.Set("Content-Type", "application/json")
			}
		}
	}
	if err != nil {
		return &Result{Error: err}
	}

	for i := 1; i < len(headers); i += 2 {
		if headers[i-1] == "Host" {
			req.Host = headers[i]
		} else {
			req.Header.Set(headers[i-1], headers[i])
		}
	}

	for k, v := range cp.GlobalHeaders {
		req.Header.Set(k, v)
	}

	res, err := cp.pool.Do(req)
	if err != nil {
		return &Result{Error: err}
	}
	result, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return &Result{Error: err}
	}
	res.Body.Close()

	return &Result{data: result, Response: res}
}

func (cp *ClientPool) getRealIp(request *http.Request) string {
	return base.StringIf(request.Header.Get(cp.XRealIpName) != "", request.Header.Get(cp.XRealIpName), request.RemoteAddr[0:strings.IndexByte(request.RemoteAddr, ':')])
}

func (rs *Result) String() string {
	if rs.data == nil {
		return ""
	}
	return string(rs.data)
}

func (rs *Result) Bytes() []byte {
	return rs.data
}

func (rs *Result) Map() map[string]interface{} {
	tr := make(map[string]interface{})
	rs.To(&tr)
	return tr
}

func (rs *Result) Arr() []interface{} {
	tr := make([]interface{}, 0)
	rs.To(&tr)
	return tr
}

func (rs *Result) ToAction(result interface{}) string {
	var actionStart = -1
	var actionEnd = -1
	var resultStart = -1
	var resultEnd = -1
	for i, c := range rs.data {
		if actionStart == -1 && c == '"' {
			actionStart = i
		}
		if actionEnd == -1 && actionStart != -1 && c == '"' {
			actionEnd = i
		}
		if resultStart == -1 && actionEnd != -1 && c != '\r' && c != '\n' && c != '\t' && c != ' ' && c != ',' {
			resultStart = i
			break
		}
	}

	if resultStart != -1 {
		for i := len(rs.data); i >= 0; i-- {
			c := rs.data[i]
			if c != '\r' && c != '\n' && c != '\t' && c != ' ' && c != ']' {
				resultEnd = i
				break
			}
		}
	}

	if resultEnd == -1 || resultEnd <= resultStart {
		return ""
	}

	convertBytesToObject(rs.data[resultStart:resultEnd-resultStart], result)
	return string(rs.data[actionStart: actionEnd-actionStart])
}

func (rs *Result) To(result interface{}) error {
	return convertBytesToObject(rs.data, result)
}

func convertBytesToObject(data []byte, result interface{}) error {
	var err error = nil
	if data == nil {
		return fmt.Errorf("no result")
	}

	t := reflect.TypeOf(result)
	v := reflect.ValueOf(result)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}
	if t.Kind() == reflect.Map || (t.Kind() == reflect.Slice && t.Elem().Kind() != reflect.Uint8) {
		err = json.Unmarshal(data, result)
	} else if t.Kind() == reflect.Struct {
		tr := new(map[string]interface{})
		err = json.Unmarshal(data, tr)
		if err == nil {
			err = mapstructure.WeakDecode(tr, result)
		}
	}
	return err
}
