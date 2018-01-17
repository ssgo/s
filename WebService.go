package s

import (
	"encoding/json"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/ssgo/base"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"
)

type webServiceType struct {
	authLevel     uint
	pathMatcher   *regexp.Regexp
	pathArgs      []string
	isService     bool
	parmsNum      int
	inType        reflect.Type
	inIndex       int
	headersType   reflect.Type
	headersIndex  int
	requestIndex  int
	responseIndex int
	callerIndex   int
	funcType      reflect.Type
	funcValue     reflect.Value
}

var webServices = make(map[string]*webServiceType)
var regexWebServices = make(map[string]*webServiceType)

var inFilters = make([]func(*map[string]interface{}, *map[string]string, *http.Request, *http.ResponseWriter) interface{}, 0)
var outFilters = make([]func(*map[string]interface{}, *map[string]string, *http.Request, *http.ResponseWriter, interface{}) (interface{}, bool), 0)

var webAuthChecker func(uint, *string, *map[string]interface{}, *map[string]string) bool

// 注册服务
func Register(authLevel uint, name string, service interface{}) {
	s, err := makeCachedService(service)
	if err != nil {
		log.Printf("ERROR	%s	%s	", name, err)
		return
	}

	s.authLevel = authLevel
	finder, err := regexp.Compile("\\{(.+?)\\}")
	if err == nil {
		keyName := regexp.QuoteMeta(name)
		finds := finder.FindAllStringSubmatch(name, 20)
		for _, found := range finds {
			keyName = strings.Replace(keyName, regexp.QuoteMeta(found[0]), "(.+?)", 1)
			s.pathArgs = append(s.pathArgs, found[1])
		}
		if len(s.pathArgs) > 0 {
			s.pathMatcher, _ = regexp.Compile("^" + keyName + "$")
			regexWebServices[name] = s
		}
	}
	if s.pathMatcher == nil {
		webServices[name] = s
	}
}

// 设置前置过滤器
func SetInFilter(filter func(in *map[string]interface{}, headers *map[string]string, request *http.Request, response *http.ResponseWriter) (out interface{})) {
	inFilters = append(inFilters, filter)
}

// 设置后置过滤器
func SetOutFilter(filter func(in *map[string]interface{}, headers *map[string]string, request *http.Request, response *http.ResponseWriter, out interface{}) (newOut interface{}, isOver bool)) {
	outFilters = append(outFilters, filter)
}

func RegisterWebAuthChecker(authChecker func(authLevel uint, url *string, request *map[string]interface{}, headers *map[string]string) bool) {
	webAuthChecker = authChecker
}

func doWebService(service *webServiceType, request *http.Request, response *http.ResponseWriter, args *map[string]interface{}, headers *map[string]string, startTime *time.Time) {
	var result interface{} = nil
	for _, filter := range inFilters {
		result = filter(args, headers, request, response)
		if result != nil {
			break
		}
	}

	// 反射调用
	if result == nil {
		// 生成参数
		var parms = make([]reflect.Value, service.parmsNum)
		if service.inType != nil {
			in := reflect.New(service.inType).Interface()
			mapstructure.WeakDecode(*args, in)
			parms[service.inIndex] = reflect.ValueOf(in).Elem()
		}
		if service.headersType != nil {
			inHeaders := reflect.New(service.headersType).Interface()
			mapstructure.WeakDecode(*headers, inHeaders)
			parms[service.headersIndex] = reflect.ValueOf(inHeaders).Elem()
		}
		if service.requestIndex >= 0 {
			parms[service.requestIndex] = reflect.ValueOf(request)
		}
		if service.responseIndex >= 0 {
			parms[service.responseIndex] = reflect.ValueOf(response)
		}
		if service.callerIndex >= 0 {
			caller := &Caller{headers: []string{"S-Unique-Id", request.Header.Get("S-Unique-Id")}}
			parms[service.callerIndex] = reflect.ValueOf(caller)
		}
		for i, parm := range parms {
			if parm.Kind() == reflect.Invalid {
				parms[i] = reflect.New(service.funcType.In(i)).Elem()
			}
		}
		outs := service.funcValue.Call(parms)
		if len(outs) > 0 {
			result = outs[0].Interface()
		} else {
			result = ""
		}
	}

	// 后置过滤器
	for _, filter := range outFilters {
		newResult, done := filter(args, headers, request, response, result)
		if newResult != nil {
			result = newResult
		}
		if done {
			break
		}
	}

	// 返回结果
	outType := reflect.TypeOf(result)
	if outType.Kind() == reflect.Ptr {
		outType = outType.Elem()
	}
	var outBytes []byte
	isJson := false
	if outType.Kind() != reflect.String && (outType.Kind() != reflect.Slice || outType.Elem().Kind() != reflect.Uint8) {
		outBytes = makeBytesResult(result)
		isJson = true
	} else if outType.Kind() == reflect.String {
		outBytes = []byte(result.(string))
	} else {
		outBytes = result.([]byte)
	}
	(*response).Write(outBytes)

	// 记录访问日志
	if recordLogs {
		usedTime := float32(time.Now().UnixNano()-startTime.UnixNano()) / 1e6
		byteArgs, _ := json.Marshal(*args)
		uniqueId := (*headers)["SUniqueId"]
		delete(*headers, "SUniqueId")
		if (*headers)["AccessToken"] != ""{
			(*headers)["AccessToken"] = (*headers)["AccessToken"][0:5]+"*******"
		}
		byteHeaders, _ := json.Marshal(*headers)
		if len(outBytes) > 1024 {
			outBytes = outBytes[0:1024]
		}
		if !isJson {
			makePrintable(outBytes)
		}
		log.Printf("ACCESS	%s	%s	%s	%.6f	%s	%s	%s	%s", request.RemoteAddr, uniqueId, request.RequestURI, usedTime, string(byteArgs), string(byteHeaders), string(outBytes), request.Proto)
	}
}

func makePrintable(data []byte){
	n := len(data)
	for i := 0; i < n; i++ {
		c := data[i]
		if c == '\t' || c == '\n' || c == '\r' {
			data[i] = ' '
		} else if c < 32 || c > 126 {
			data[i] = '?'
		}
	}
}

func makeCachedService(matchedServie interface{}) (*webServiceType, error) {
	// 类型或参数返回值个数不对
	funcType := reflect.TypeOf(matchedServie)
	if funcType.Kind() != reflect.Func {
		return nil, fmt.Errorf("Bad Service")
	}

	// 参数类型不对
	targetService := new(webServiceType)
	targetService.parmsNum = funcType.NumIn()
	targetService.inIndex = -1
	targetService.headersIndex = -1
	targetService.requestIndex = -1
	targetService.responseIndex = -1
	targetService.callerIndex = -1
	for i := 0; i < targetService.parmsNum; i++ {
		t := funcType.In(i)
		if t.String() == "*http.Request" {
			targetService.requestIndex = i
		} else if t.String() == "*http.ResponseWriter" {
			targetService.responseIndex = i
		} else if t.String() == "*s.Caller" {
			targetService.callerIndex = i
		}else if t.Kind() == reflect.Struct {
			if targetService.inType == nil {
				targetService.inIndex = i
				targetService.inType = t
			} else if targetService.headersType == nil {
				targetService.headersIndex = i
				targetService.headersType = t
			}
		}
	}

	if funcType.NumIn() > 0 {
		// 返回值类型不对
		outType := funcType.Out(0)
		if outType.Kind() == reflect.Ptr {
			outType = outType.Elem()
		}
		if outType.Kind() != reflect.String && (outType.Kind() != reflect.Slice || outType.Elem().Kind() != reflect.Uint8) {
			targetService.isService = false
			outType = funcType.Out(0)
		} else {
			targetService.isService = true
		}
	}

	targetService.funcType = funcType
	targetService.funcValue = reflect.ValueOf(matchedServie)
	return targetService, nil
}

func makeBytesResult(data interface{}) []byte {
	bytesResult, err := json.Marshal(data)
	if err != nil {
		bytesResult = []byte("{}")
	}
	base.FixUpperCase(bytesResult)
	return bytesResult
}
