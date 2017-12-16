# Go的一个服务框架

核心思想是使用一个 Struct 或 Map 来接收 传入的参数、请求头、上下文 等

返回标准的 code、message、自定义的 data

## 快速构建一个服务

写好服务处理函数

```go
func Index() string {
	return "Hello World!"
}

func Login(in struct {
    Account   string
    Password  string
}) (int, string, bool) {
	if in.Account == "admin" && in.Password == "admin123" {
    	return 200, "Logined", true
    }
    return 403, "No Access", false
}
```

注册服务并启动

```go
service.Register("/", Index)
service.Register("/login", Login)
service.Start()
```

即可快速构建出一个可运行的服务

##测试用例

与创建服务类似的调用，可直接完成测试工作

```go
func TestIndex(tt *testing.T) {
	t := service.T(tt)
	service.Register("/", Index)
	service.StartTestService()
	defer service.StopTestService()
	_, result, err := service.TestGet("/")
	t.Test( strings.Contains(string(result), "Hello World!"), "Index", string(result), err)
}

```

## 配置文件

项目根目录放置一个 service.json

```json
{
    "listen": ":8080"
}
```

也可以在环境变量中设置 SERVICE_LISTEN = ":8080" 实现

## API

```go
// 注册服务
func Register(name string, service interface{})

// 注册以正则匹配的服务
func RegisterByRegex(name string, service interface{})

// 设置上下文内容，可以在服务函数的参数中直接得到并使用
func SetContext(name string, context interface{})

// 设置前置过滤器
func SetInFilter(filter func(map[string]interface{}) *Result) 

// 设置后置过滤器
func SetOutFilter(filter func(map[string]interface{}, *Result) *Result)

// 启动服务
func Start()
```



