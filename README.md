# Go的一个服务框架

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
  c.Call("s1", "/"+in.Name+"/fullName", nil).To(&out)
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
  "listen": ":80",
  "RwTimeout": 5000,
  "KeepaliveTimeout": 5000,
  "CallTimeout": 5000,
  "logFile": "",
  "certFile": "",
  "keyFile": "",

  "registry": "discover:15",
  "registryPrefix": "",
  "app": "demo",
  "weight": 1,
  "AccessTokens": {
    "hasfjlkdlasfsa": 1,
    "fdasfsadfdsa": 2,
    "9ifjjabdsadsa": 2
  },
  "calls": {
    "user": {}
    "news": {"AccessToken": "hasfjlkdlasfsa", "Timeout": 5000}
  }
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
func SetInFilter(filter func(in *map[string]interface{}, headers *map[string]string, request *http.Request, response *http.ResponseWriter) (out interface{})) {}

// 设置后置过滤器
func SetOutFilter(filter func(in *map[string]interface{}, headers *map[string]string, request *http.Request, response *http.ResponseWriter, out interface{}) (newOut interface{}, isOver bool)) {}

// 注册身份认证模块
func SetWebAuthChecker(authChecker func(authLevel uint, url *string, request *map[string]interface{}, headers *map[string]string) bool) {}

// 启动HTTP/1.1服务
func Start1() {}

// 启动HTTP/2.0服务（若未配置证书将工作在No SSL模式）
func Start() {}

// 异步方式启动HTTP/2.0服务（）
func AsyncStart() string {}
// 停止以异步方式启动的服务（同步方式启动的服务不需要调用Stop，会根据kill信号自己关闭）
func Stop() {}
// 停止以异步方式启动的服务后等待各种子线程结束
func WaitForAsync() {}

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

