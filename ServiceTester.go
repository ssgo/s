package service

import (
	"net/http"
	"io/ioutil"
	"net/http/httptest"
	"encoding/json"
	"bytes"
	"testing"
	"fmt"
	"io"
)

var testServer *httptest.Server
var testHeaders = make(map[string]string)
func StartTestService() *httptest.Server{
	testServer = httptest.NewServer(http.Handler(&routeHandler{}))
	//if recordLogs {fmt.Println()}
	//fmt.Println("Start test service\n")
	return testServer
}

func SetTestHeader(k string, v string){
	testHeaders[k] = v
}

func testRequest(method string, path string, body []byte) (*http.Response, []byte, error) {
	client := &http.Client{}
	var bodyReader io.Reader = nil
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, testServer.URL+path, bodyReader)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range testHeaders {
		req.Header.Add(k, v)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()

	result, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, nil, err
	}
	res.Body.Close()

	return res, result, nil
}

func TestGet(path string) (*http.Response, []byte, error) {
	return testRequest("GET", path, nil)
}

//func TestPost(path string, args map[string]string) ([]byte, error) {
//	return testRequest("POST", path, nil)
//}

func TestService(path string, args map[string]interface{}) (int, string, interface{}) {

	argsObjectBytes, _ := json.Marshal(args)

	_, result, err := testRequest("POST", path, argsObjectBytes)
	if err != nil {
		return 500, err.Error(), nil
	}

	resultObject := make(map[string]interface{})
	err = json.Unmarshal(result, &resultObject)
	if err != nil {
		return 500, err.Error(), nil
	}

	return int(resultObject["code"].(float64)), resultObject["message"].(string), resultObject["data"]
}

func StopTestService() {
	testServer.Close()
	//if recordLogs {fmt.Println()}
	//fmt.Println("\n\nStop test service")
}

type Testing struct {
	tt *testing.T
	tb *testing.B
}

func T(tt *testing.T) *Testing {
	t := new(Testing)
	t.tt = tt
	return t
}
func B(tb *testing.B) *Testing {
	t := new(Testing)
	t.tb = tb
	return t
}
func (t *Testing) Test(tests bool, comment string, addons ...interface{}) {
	if !tests {
		fmt.Println("  \x1b[0;41m失败\x1b[0m", comment, addons)
		if t.tt != nil {
			t.tt.Error(comment, addons)
			panic(comment)
		}else if t.tb != nil {
			t.tb.Error(comment, addons)
			panic(comment)
		}
	}else{
		fmt.Println("  \x1b[0;42m成功\x1b[0m", comment)
	}
}
