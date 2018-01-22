package s

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/net/http2"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"time"
)

type ClientPool struct {
	pool          *http.Client
	globalHeaders map[string]string
}

type Result struct {
	Error    error
	Response *http.Response
	data     []byte
}

func GetClient() *ClientPool {
	clientConfig := &http.Client{Transport: &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		}}}
	if config.CallTimeout > 0 {
		clientConfig.Timeout = time.Duration(config.CallTimeout) * time.Millisecond
	}
	return &ClientPool{pool: clientConfig, globalHeaders:map[string]string{"User-Agent": "S-Client/2.0"}}
}
func GetClient1() *ClientPool {
	return &ClientPool{pool: &http.Client{}, globalHeaders:map[string]string{"User-Agent": "S-Client/1.1"}}
}

func (cp *ClientPool) SetGlobalHeader(k, v string) {
	if v == "" {
		delete(cp.globalHeaders, k)
	} else {
		cp.globalHeaders[k] = v
	}
}

func (cp *ClientPool) Get(url string, headers ... string) *Result {
	return cp.Do("GET", url, nil, headers...)
}
func (cp *ClientPool) Post(url string, data interface{}, headers ... string) *Result {
	return cp.Do("POST", url, data, headers...)
}
func (cp *ClientPool) Put(url string, data interface{}, headers ... string) *Result {
	return cp.Do("PUT", url, data, headers...)
}
func (cp *ClientPool) Delete(url string, data interface{}, headers ... string) *Result {
	return cp.Do("DELETE", url, data, headers...)
}
func (cp *ClientPool) Head(url string, data interface{}, headers ... string) *Result {
	return cp.Do("HEAD", url, data, headers...)
}
func (cp *ClientPool) Do(method, url string, data interface{}, headers ... string) *Result {
	var req *http.Request
	var err error
	if data == nil {
		req, err = http.NewRequest(method, url, nil)
	} else {
		var bytesData []byte
		bytesData, err = json.Marshal(data)
		if err == nil {
			req, err = http.NewRequest(method, url, bytes.NewReader(bytesData))
			req.Header.Add("Content-Type", "application/json")
		}
	}
	if err != nil {
		return &Result{Error: err}
	}

	for k, v := range cp.globalHeaders {
		req.Header.Add(k, v)
	}

	for i := 1; i < len(headers); i += 2 {
		req.Header.Add(headers[i-1], headers[i])
	}
//t1 := time.Now()
	res, err := cp.pool.Do(req)
//log.Print(" ((((((((((	", url, "	", float32(time.Now().UnixNano()-t1.UnixNano()) / 1e6)
	if err != nil {
		return &Result{Error: err}
	}
	defer res.Body.Close()

	result, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return &Result{Error: err}
	}
	res.Body.Close()

	return &Result{data: result, Response: res}
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
		return fmt.Errorf("No Result")
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
