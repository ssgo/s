package s

import (
	"fmt"
	"github.com/ssgo/config"
	"net/http"
	"reflect"
	"testing"
)

//var testServer *httptest.Server
//var testHeaders = make(map[string]string)
//
//func StartTestService() *httptest.Server {
//	initConfig()
//	testServer = httptest.NewServer(http.Handler(&routeHandler{}))
//	//if recordLogs {fmt.Println()}
//	//fmt.Println("Start test service\n")
//	return testServer
//}
//
//func SetTestHeader(k string, v string) {
//	testHeaders[k] = v
//}

func ResetAllSets() {
	config.ResetConfigEnv()
	Config = serviceConfig{}
	inited = false

	rewrites = make(map[string]*rewriteInfo)
	regexRewrites = make([]*rewriteInfo, 0)
	proxies = make(map[string]*proxyInfo, 0)
	regexProxies = make([]*proxyInfo, 0)
	statics = make(map[string]*string)
	usedSessionIdKey = ""
	//sessionCreator = nil
	//sessionObjects = map[*http.Request]map[reflect.Type]interface{}{}
	injectObjects = map[reflect.Type]interface{}{}

	webServices = make(map[string]*webServiceType)
	regexWebServices = make([]*webServiceType, 0)
	inFilters = make([]func(map[string]interface{}, *http.Request, *http.ResponseWriter) interface{}, 0)
	outFilters = make([]func(map[string]interface{}, *http.Request, *http.ResponseWriter, interface{}) (interface{}, bool), 0)

	websocketServices = make(map[string]*websocketServiceType)
	regexWebsocketServices = make([]*websocketServiceType, 0)
	webAuthChecker = nil
	webSocketActionAuthChecker = nil
}

//func testRequest(method string, path string, body []byte) (*http.Response, []byte, error) {
//	client := &http.Client{}
//	var bodyReader io.Reader = nil
//	if body != nil {
//		bodyReader = bytes.NewReader(body)
//	}
//	req, err := http.NewRequest(method, testServer.URL+path, bodyReader)
//	if err != nil {
//		return nil, nil, err
//	}
//
//	for k, v := range testHeaders {
//		req.Header.Add(k, v)
//	}
//	res, err := client.Do(req)
//	if err != nil {
//		return nil, nil, err
//	}
//	defer res.Body.Close()
//
//	result, err := ioutil.ReadAll(res.Body)
//	if err != nil {
//		return nil, nil, err
//	}
//	res.Body.Close()
//
//	return res, result, nil
//}
//
//func TestGet(path string) (*http.Response, []byte, error) {
//	return testRequest("GET", path, nil)
//}
//
////func TestPost(path string, args map[string]string) ([]byte, error) {
////	return testRequest("POST", path, nil)
////}
//
//func TestService(path string, args Map) interface{} {
//
//	argsObjectBytes, _ := json.Marshal(args)
//
//	_, result, err := testRequest("POST", path, argsObjectBytes)
//	if err != nil {
//		fmt.Println("testRequest", err)
//		return nil
//	}
//
//	var resultObject interface{}
//	err = json.Unmarshal(result, &resultObject)
//	if err != nil {
//		fmt.Println("Unmarshal", err)
//		return nil
//	}
//
//	return resultObject
//}
//
//func StopTestService() {
//	testServer.Close()
//	//if recordLogs {fmt.Println()}
//	//fmt.Println("\n\nStop test service")
//}

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
		} else if t.tb != nil {
			t.tb.Error(comment, addons)
			panic(comment)
		}
	} else {
		fmt.Println("  \x1b[0;42m成功\x1b[0m", comment)
	}
}
