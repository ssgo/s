# Go的一个服务框架

[![Build Status](https://travis-ci.org/ssgo/s.svg?branch=master)](https://travis-ci.org/ssgo/s)

核心思想是使用 Struct 传入的参数，根据使用需求注入参数

并且能以非常简单的方式快速部署成为微服务群



## 快速构建一个服务

```go
package main

import "github.com/ssgo/s"

func main() {
	s.Register(0, "/", func() string {
		return "Hello\n"
	})
	s.Start1()
}
```

即可快速构建出一个可运行的服务

```shell
export SERVICE_LISTEN=:8080
go run hello.go
```

服务默认使用随机端口启动，若要指定端口可设置环境变量，或使用配置文件 /service.json



## 服务发现

#### Service

```go
package main

import "github.com/ssgo/s"

func getFullName(in struct{ Name string }) (out struct{ FullName string }) {
  out.FullName = in.Name + " Lee"
  return
}

func main() {
  s.Register(1, "/{name}/fullName", getFullName)
  s.Start()
}
```

```shell
export SERVICE_APP=s1
export SERVICE_ACCESSTOKENS='{"aabbcc":1}'
go run service.go
```

该服务工作在认证级别1上，派发了一个令牌 “aabbcc”，不带该令牌的请求将被拒绝

s.Start() 将会工作在 HTTP/2.0 No SSL 协议上（服务间通讯默认都使用 HTTP/2.0 No SSL 协议）

并且自动连接本机默认的redis服务，并注册一个叫 s1 的服务（如需使用其他可以参考redis的配置）

可运行多个实例，Gateway访问时将会自动负载均衡到某一个节点

#### Gateway

```go
package main

import "github.com/ssgo/s"

func getInfo(in struct{ Name string }, c *s.Caller) (out struct{ FullName string }) {
  c.Get("s1", "/"+in.Name+"/fullName", nil).To(&out)
  return
}

func main() {
  s.Register(0, "/{name}", getInfo)
  s.Start1()
}
```

```shell
export SERVICE_LISTEN=:8080
export SERVICE_CALLS='{"s1": {"accessToken": "aabbcc"}}'
go run gateway.go
```

该服务工作在认证级别0上，可以随意访问

s.Start1() 将会工作在 HTTP/1.1 协议上（方便直接测试）

getInfo 方法中调用 s1 时会根据 redis 中注册的节点信息负载均衡到某一个节点

所有调用 s1 服务的请求都会自动带上 "aabbcc" 这个令牌以获得相应等级的访问权限



## 配置

可在项目根目录放置一个 service.json

```json
{
  "listen": ":",
  "httpVersion": 2,
  "rwTimeout": 5000,
  "keepaliveTimeout": 15000,
  "callTimeout": 10000,
  "logFile": "",
  "logLevel": "info",
  "noLogGets": false,
  "noLogHeaders": "Accept,Accept-Encoding,Accept-Language,Cache-Control,Pragma,Connection,Upgrade-Insecure-Requests",
  "encryptLogFields": "password,secure,token,accessToken",
  "noLogInputFields": false,
  "logInputArrayNum": 0,
  "logOutputFields": "code,message",
  "logOutputArrayNum": 2,
  "logWebsocketAction": false,
  "compress": true,
  "xUniqueId": "X-Unique-Id",
  "xForwardedForName": "X-Forwarded-For",
  "xRealIpName": "X-Real-Ip",
  "certFile": "",
  "keyFile": "",
  "registry": "discover:15",
  "registryCalls": "discover:15",
  "registryPrefix": "",
  "app": "",
  "weight": 1,
  "accessTokens": {
    "hasfjlkdlasfsa": 1,
    "fdasfsadfdsa": 2,
    "9ifjjabdsadsa": 2
  },
  "calls": {
    "user": {}
    "news": {"accessToken": "hasfjlkdlasfsa", "timeout": 5000, "httpVersion": 2, "withSSL": false}
  },
  "callRetryTimes": 10
}
```

配置内容也可以同时使用环境变量设置（优先级高于配置文件）

例如：

```shell
export SERVICE='{"listen": ":80", "app": "s1"}'
export SERVICE_LISTEN=10.34.22.19:8001
export SERVICE_CALLS_NEWS_ACCESSTOKEN=real_token
```



## API

```go
// 注册服务
func Register(authLevel uint, name string, serviceFunc interface{}) {}

// 注册以正则匹配的服务
func RegisterByRegex(name string, service interface{}){}

// 设置前置过滤器
func SetInFilter(filter func(in *map[string]interface{}, request *http.Request, response *http.ResponseWriter) (out interface{})) {}

// 设置后置过滤器
func SetOutFilter(filter func(in *map[string]interface{}, request *http.Request, response *http.ResponseWriter, out interface{}) (newOut interface{}, isOver bool)) {}

// 注册身份认证模块
func SetAuthChecker(authChecker func(authLevel uint, url *string, request *map[string]interface{}) bool) {}

// 启动HTTP/1.1服务
func Start1() {}

// 启动HTTP/2.0服务（若未配置证书将工作在No SSL模式）
func Start() {}

// 异步方式启动HTTP/2.0服务（）
func AsyncStart() *AsyncServer {}
// 异步方式启动HTTP/1.1服务（）
func AsyncStart1() *AsyncServer {}

// 停止以异步方式启动的服务后等待各种子线程结束
func (as *AsyncServer) Stop() {}

// 调用异步方式启动的服务
func (as *AsyncServer) Get(path string, headers ... string) *Result {}
func (as *AsyncServer) Post(path string, data interface{}, headers ... string) *Result {}
func (as *AsyncServer) Put(path string, data interface{}, headers ... string) *Result {}
func (as *AsyncServer) Head(path string, data interface{}, headers ... string) *Result {}
func (as *AsyncServer) Delete(path string, data interface{}, headers ... string) *Result {}
func (as *AsyncServer) Do(path string, data interface{}, headers ... string) *Result {}

```



## 服务发现 Discover

基于 Http Header 传递 SessionId（不推荐使用Cookie）
使用 SetSession 设置的对象可以在服务方法中直接使用相同类型获得对象，一般是在 AuthChecker 或者 InFilter 中设置

```shell

export SERVICE_REGISTRY =       // 配置注册服务使用的 Redis 连接配置（redis.json 或 环境变量）
export SERVICE_REGISTRYPREFIX = // 指定一个存储注册信息前缀
export SERVICE_APP =            // 指定应用名称，存在此选项将运行在服务模式
export SERVICE_WEIGHT =         // 服务的权重
export SERVICE_ACCESSTOKENS =   // 设置允许访问该服务的令牌
export SERVICE_CALLS =          // 设置将会访问的服务，存在此选项将运行在客户模式

```

```go

// 调用已注册的服务，根据权重负载均衡
func (caller *Caller) Get(app, path string, headers ... string) *Result {}
func (caller *Caller) Post(app, path string, data interface{}, headers ... string) *Result {}
func (caller *Caller) Put(app, path string, data interface{}, headers ... string) *Result {}
func (caller *Caller) Head(app, path string, data interface{}, headers ... string) *Result {}
func (caller *Caller) Delete(app, path string, data interface{}, headers ... string) *Result {}
func (caller *Caller) Do(app, path string, data interface{}, headers ... string) *Result {}

// 指定节点调用已注册的服务，并返回本次使用的节点
func (caller *Caller) DoWithNode(method, app, withNode, path string, data interface{}, headers ... string) (*Result, string) {}

// 设置一个负载均衡算法
func SetLoadBalancer(lb LoadBalancer) {}

type LoadBalancer interface {

	// 每个请求完成后提供信息
	Response(node *NodeInfo, err error, response *http.Response, responseTimeing int64)

	// 请求时根据节点的得分取最小值发起请求
	Next(nodes []*NodeInfo, request *http.Request) *NodeInfo
}

```



## 日志输出

使用json格式输出日志

```go
func Debug(logType string, data Map) {}

func Info(logType string, data Map) {}

func Warning(logType string, data Map) {}

func Error(logType string, data Map) {}

func Log(logLevel LogLevelType, logType string, data Map) {}

func TraceLog(logLevel LogLevelType, logType string, data Map) {}

```



## Session 和 注入

基于 Http Header 传递 SessionId（不推荐使用Cookie）
使用 SetSession 设置的对象可以在服务方法中直接使用相同类型获得对象，一般是在 AuthChecker 或者 InFilter 中设置

```go
// 设置 SessionKey，自动在 Header 中产生，AsyncStart 的客户端支持自动传递
func SetSessionKey(inSessionKey string) {}

// 获取 SessionKey
func GetSessionKey() string {}

// 设置一个生命周期在 Request 中的对象，请求中可以使用对象类型注入参数方便调用
func SetSessionInject(request *http.Request, obj interface{}) {}

// 获取本生命周期中指定类型的 Session 对象
func GetSessionInject(request *http.Request, dataType reflect.Type) interface{} {}

// 设置一个注入对象，请求中可以使用对象类型注入参数方便调用
func SetInject(obj interface{}) {}

// 获取一个注入对象
func GetInject(dataType reflect.Type) interface{} {}

```


## Websocket

一个以Action为处理单位的 Websocket 封装

```go
// 注册Websocket服务
func RegisterWebsocket(authLevel uint, name string, updater *websocket.Upgrader,
	onOpen interface{},
	onClose interface{},
	decoder func(data interface{}) (action string, request *map[string]interface{}, err error),
	encoder func(action string, data interface{}) interface{}) *ActionRegister {}

// 注册Websocket Action
func (ar *ActionRegister) RegisterAction(authLevel uint, actionName string, action interface{}) {}

// 注册针对 Action 的认证模块
func SetActionAuthChecker(authChecker func(authLevel uint, url *string, action *string, request *map[string]interface{}, sess interface{}) bool) {}

```



## Document 自动生成接口文档

```go
// 生成文档数据
func MakeDocument() []Api {}

// 生成文档并存储到 json 文件中
func MakeJsonDocumentFile(file string) {

// 生成文档并存储到 html 文件中，使用默认html模版
func MakeHtmlDocumentFile(title, toFile string) string {}

// 生成文档并存储到 html 文件中，使用指定html模版
func MakeHtmlDocumentFromFile(title, toFile, fromFile string) string {}

```

## Document 使用命令行创建文档（假设编译好的文件为 ./server）

```shell
// 直接输出 json 格式文档
./server doc

// 生成 json 格式文档
./server doc xxx.json

// 生成 html 格式文档，使用默认html模版
./server doc xxx.html

// 生成 html 格式文档，使用指定html模版
./server doc xxx.html tpl.html

```