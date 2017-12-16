package service

import (
	"github.com/ssgo/base"
	"net/http"
	"strings"
	"io/ioutil"
	"encoding/json"
	"log"
	"time"
	"github.com/mitchellh/mapstructure"
	"reflect"
	"regexp"
)

type cachedService struct {
	isService bool
	inType    reflect.Type
	inValue   reflect.Value
	funcType  reflect.Type
	funcValue reflect.Value
}

var services = make(map[string]interface{})
var regexServices = make(map[string]interface{})
var inFilters = make([]func(map[string]interface{}) *Result, 0)
var outFilters = make([]func(map[string]interface{}, *Result) *Result, 0)
var contexts = make(map[string]interface{})
var cachedServices = make(map[string]*cachedService)
var recordLogs = true

var config = struct {
	Listen string
}{}

func init() {
	base.LoadConfig("service", &config)
	if config.Listen == "" {
		config.Listen = ":8001"
	}
}

// 注册服务
func Register(name string, service interface{}) {
	services[name] = service
}

// 注册以正则匹配的服务
func RegisterByRegex(name string, service interface{}) {
	regexServices[name] = service
}

// 设置上下文内容，可以在服务函数的参数中直接得到并使用
func SetContext(name string, context interface{}) {
	contexts[name] = context
}

// 设置前置过滤器
func SetInFilter(filter func(map[string]interface{}) *Result) {
	inFilters = append(inFilters, filter)
}

// 设置后置过滤器
func SetOutFilter(filter func(map[string]interface{}, *Result) *Result) {
	outFilters = append(outFilters, filter)
}

// 启动服务
func Start() {
	http.Handle("/", &routeHandler{})
	err := http.ListenAndServe(config.Listen, nil)
	if err != nil {
		log.Println(err)
	}
}

func EnableLogs(enabled bool) {
	recordLogs = enabled
}

func ResetAllSets() {
	services = make(map[string]interface{})
	regexServices = make(map[string]interface{})
	inFilters = make([]func(map[string]interface{}) *Result, 0)
	outFilters = make([]func(map[string]interface{}, *Result) *Result, 0)
	contexts = make(map[string]interface{})
	cachedServices = make(map[string]*cachedService)
	recordLogs = true
}

type routeHandler struct{}
func (*routeHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	startTime := time.Now()

	// 获取路径
	requestPath := request.RequestURI
	pos := strings.LastIndex(requestPath, "?")
	if pos != -1 {
		requestPath = requestPath[0:pos]
	}

	s := cachedServices[requestPath]
	if s == nil {
		// 未注册的路径
		var matchedServie interface{} = services[requestPath]
		if matchedServie == nil {
			for k, v := range regexServices{
				matched, err := regexp.MatchString(k, requestPath)
				if err == nil && matched {
					matchedServie = v
				}
			}

			if matchedServie == nil {
				response.WriteHeader(404)
				return
			}
		}

		// 类型或参数返回值个数不对
		funcType := reflect.TypeOf(matchedServie)
		if funcType.Kind() != reflect.Func || (funcType.NumIn() != 1 && funcType.NumIn() != 0) || (funcType.NumOut() != 3 && funcType.NumOut() != 1) {
			response.Write(makeBytesResult(510, "Bad Service", nil))
			return
		}

		// 参数类型不对
		var inType reflect.Type = nil
		if funcType.NumIn() == 1 {
			inType = funcType.In(0)
			if inType.Kind() != reflect.Struct && inType.Kind() != reflect.Map {
				response.Write(makeBytesResult(510, "Bad Service Inputs", nil))
				return
			}
		}

		// 返回值类型不对
		s = new(cachedService)
		if funcType.NumOut() == 1 {
			s.isService = false
			outType := funcType.Out(0)
			if outType.Kind() != reflect.String && (outType.Kind() != reflect.Slice || outType.Elem().Kind() != reflect.Uint8) {
				response.Write(makeBytesResult(510, "Bad Service Outputs", nil))
				return
			}
		}else{
			s.isService = true
			outCodeType := funcType.Out(0)
			outMessageType := funcType.Out(1)
			if outCodeType.Kind() != reflect.Int || outMessageType.Kind() != reflect.String {
				response.Write(makeBytesResult(510, "Bad Service Outputs", nil))
				return
			}
		}
		s.inType = inType
		if inType != nil {
			s.inValue = reflect.New(inType)
		}
		s.funcType = funcType
		s.funcValue = reflect.ValueOf(matchedServie)
		cachedServices[requestPath] = s
	}

	args := make(map[string]interface{})

	// GET POST
	request.ParseForm()
	for k, v := range request.Form {
		if len(v) > 1 {
			args[k] = v
		} else {
			args[k] = v[0]
		}
	}
	//[110 117 108 108]
	// POST JSON
	bodyBytes, _ := ioutil.ReadAll(request.Body)
	defer request.Body.Close()
	if len(bodyBytes) > 1 && bodyBytes[0] == 123 {
		json.Unmarshal(bodyBytes, &args)
	}

	// Headers
	for k, v := range request.Header {
		headerKey := strings.Replace(k, "-", "", -1)
		if len(v) > 1 {
			args[headerKey] = v
		} else {
			args[headerKey] = v[0]
		}
	}
	args["HttpRequestPath"] = requestPath
	args["HttpRequest"] = request
	args["HttpResponse"] = response

	// Contexts
	for k, v := range contexts {
		args[k] = v
	}

	// 前置过滤器
	var result *Result = nil
	for _, filter := range inFilters {
		result = filter(args)
		if result != nil {
			break
		}
	}

	// 反射调用
	if result == nil {
		// 生成参数
		var parms []reflect.Value
		if s.inType != nil {
			if s.inType.Kind() == reflect.Struct {
				in := s.inValue.Interface()
				mapstructure.WeakDecode(args, in)
				parms = []reflect.Value{reflect.ValueOf(in).Elem()}
			} else {
				parms = []reflect.Value{reflect.ValueOf(args)}
			}
		}

		outs := s.funcValue.Call(parms)

		if s.isService {
			code := int(outs[0].Int())
			message := outs[1].String()
			data := outs[2].Interface()
			result = &Result{code, message, data}
		}else{
			data := outs[0].Interface()
			result = &Result{200, "OK", data}
		}
	}

	// 后置过滤器
	for _, filter := range outFilters {
		newResult := filter(args, result)
		if newResult != nil {
			// 使用新的结果
			result = newResult
			break
		}
	}

	// 记录访问日志
	if result != nil {
		if s.isService {
			response.Write(makeBytesResult(result.Code, result.Message, result.Data))
		}else{
			var outBytes []byte
			if reflect.TypeOf(result.Data).Kind() == reflect.String {
				outBytes = []byte(result.Data.(string))
			}else{
				outBytes = result.Data.([]byte)
			}
			response.Write(outBytes)
		}
		if recordLogs {
			usedTime := float32(time.Now().Nanosecond()-startTime.Nanosecond()) / 1e9
			log.Printf("\tACCESS\t%s\t%s\t%s\t%s\t%s\t%.6f\t%d\t%s\t%s\n", request.RemoteAddr, args["Account"], args["ClientId"], args["SessionId"], request.RequestURI, usedTime, result.Code, result.Message, request.UserAgent())
		}
	}
}

type Result struct {
	Code    int
	Message string
	Data    interface{}
}

func makeBytesResult(code int, message string, data interface{}) []byte {
	bytesResult, err := json.Marshal(map[string]interface{}{
		"code":    code,
		"message": message,
		"data":    data,
	})
	if err != nil {
		bytesResult = []byte("{\"code\":511,\"message\":\"Bad Result\",\"data\":null}")
	}
	n := len(bytesResult)
	inObject := false
	for i := 1; i < n-1; i++ {
		if bytesResult[i] == '{' {
			inObject = true
		} else if bytesResult[i] == '}' {
			inObject = false
		}
		if bytesResult[i] == '"' && (bytesResult[i-1] == '{' || (bytesResult[i-1] == ',' && inObject)) && (bytesResult[i+1] >= 'A' && bytesResult[i+1] <= 'Z') {
			bytesResult[i+1] += 32
		}
	}

	return bytesResult
}
