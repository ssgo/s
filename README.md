# Go的一个服务框架

[![Build Status](https://travis-ci.org/ssgo/s.svg?branch=master)](https://travis-ci.org/ssgo/s)

ssgo能以非常简单的方式快速部署成为微服务群

## 开始使用

如果go version >= 1.11，使用以下命令新建一个项目:

```shell
go mod init sshow
go mod tidy
```

1、项目建立之后，下载并安装s

```shell
go get -u github.com/ssgo/s
```

2、在代码中导入它

```shell
import "github.com/ssgo/s"
```

## 快速构建一个服务

```go
package main

import "github.com/ssgo/s"

func main() {
	s.Restful(0, "GET", "/hello", func() string{
		return "Hello ssgo\n"
	})
	s.Start1()
}
```

即可快速构建出一个可运行的服务

```shell
export SERVICE_APP=z1
export SERVICE_LISTEN=:8080
go run start.go
```

windows下环境变量不区分大小写，windows下使用：

```cmd
set service_app=z1
set service_listen=:8080
go run start.go
```

服务默认使用随机端口启动，若要指定端口可设置环境变量，或start.go目录下配置文件service.json

```json
{
  "app": "s1",
  "listen":":8081"
}
```
开发时可以使用配置文件

<font color="#FF0000">部署推荐使用容器技术设置环境变量</font>


## redis

框架服务发现机制基于redis实现，所以使用discover之前，先要准备一个redis服务

默认使用127.0.0.1:6379，db默认为15，密码默认为空，也可以自定义配置redis.json

如果您的redis的密码如果不为空，需要使用aes加密后将密文放在配置文件password字段上，保障密码不泄露

#### 密码使用AES加密

可以使用github.com/s/redis/tests/redis_test.go中MakePasswd()方法，跑单元测试

假设密码为：ssgo-test

```shell
go test  -v -run MakePasswd YourPath/redis_test.go -args passwd "ssgo-test"
```

得到结果：

Redis encrypted `ssgo-test` is:upvNALgTxwS/xUp2Cie4tg==


也可以自己构建应用设置密码：

```go
package main

import (
	"fmt"
	"github.com/ssgo/s/redis"
)

func main() {
	testString := "ssgo-test"
	encrypted := redis.MakePasswd(testString)
	fmt.Println("Redis encrypted password is:" + encrypted)
}
```

得到AES加密后的密码放入redis.json中

```json
{
  "discover":{
  "host":"127.0.0.1:6379",
  "password":"upvNALgTxwS/xUp2Cie4tg==",
  "db":1
  }
}
```

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
export SERVICE_ACCESSTOKENS='{"s1token":1}'
go run service.go
```

windows下使用：

```cmd
set service_accesstokens={"s1token":1}
go run service.go
```

该服务工作在认证级别1上，派发了一个令牌 “s1token”，不带该令牌的请求将被拒绝

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
export SERVICE_APP=g1
export SERVICE_LISTEN=:8091
export SERVICE_CALLS='{"s1": {"accessToken": "s1token"}}'
go run gateway.go &
```

windows下使用：

```cmd
set service_app=g1
set service_listen=:8091
set service_calls={"s1": {"accessToken": "sltoken"}}
go run gateway.go
```

该服务工作在认证级别0上，可以随意访问

s.Start1() 将会工作在 HTTP/1.1 协议上（方便直接测试）

getInfo 方法中调用 s1 时会根据 redis 中注册的节点信息负载均衡到某一个节点

所有调用 s1 服务的请求都会自动带上 "sltoken" 这个令牌以获得相应等级的访问权限

## 配置

可在应用根目录放置一个 service.json

```json
{
  "listen": ":8081",
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
    "user": {},
    "news": {"accessToken": "hasfjlkdlasfsa", "timeout": 5000, "httpVersion": 2, "withSSL": false}
  },
  "callRetryTimes": 10
}
```

#### redis配置

redis的使用配置可以放在应用根目录redis.json中

```json
{
  "test": {
    "host": "127.0.0.1:6379",
    "password": "",
    "db": 1,
    "maxActive": 100,
    "maxIdles": 30,
    "idleTimeout": 0,
    "connTimeout": 3000,
    "readTimeout": 0,
    "writeTimeout": 0
  },
  "discover": {
    "…":"…"
  }
}
```

#### env配置

可以在应用根目录使用env.json综合配置(redis+service)在开发的项目：

```json
{
  "redis":{
    "discover":{
      "host":"127.0.0.1:6379",
      "password":"upvNALgTxwS/xUp2Cie4tg==",
      "db":1
    }
  },
  "service":{
    "app":"e1",
    "listen":":8081"
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

windows:

```cmd
set service={"listen": ":80", "app": "s1"}
set service_listen=10.34.22.19:8001
set service_calls_news_accesstoken=real_token
```

具体配置：

```shell
export SERVICE_REGISTRY =       // 配置注册服务使用的 Redis 连接配置（redis.json 或 环境变量）
export SERVICE_REGISTRYPREFIX = // 指定一个存储注册信息前缀
export SERVICE_APP =            // 指定应用名称，存在此选项将运行在服务模式
export SERVICE_WEIGHT =         // 服务的权重
export SERVICE_ACCESSTOKENS =   // 设置允许访问该服务的令牌
export SERVICE_CALLS =          // 设置将会访问的服务，存在此选项将运行在客户模式
export REDIS_DISCOVER_HOST=     // 设置服务发现redis服务地址
```

配置优先级顺序：

os.setEnv > cli设置环境变量(set/export) > 配置文件

## 框架常用API

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

## 基本使用

#### Restful使用GET、POST、PUT、HEAD、DELETE和OPTIONS
```go

package main

import (
	"github.com/ssgo/s"
	"net/http"
	"os"
)

type actionIn struct {
	Aaa int
	Bbb string
	Ccc string
}

func restAct(req *http.Request, in actionIn) actionIn {
	return in
}
func showFullName(in struct{ Name string }) (out struct{ FullName string }) {
	out.FullName = in.Name + " Lee."
	return
}
func main() {
	//demo演示，实际场景不推荐这样配置
	os.Setenv("service_listen", ":8301")
	//http://127.0.0.1:8301/api/echo?aaa=1&bbb=2&ccc=3
	s.Restful(0, "GET", "/api/echo", restAct)
	s.Restful(1, "POST", "/api/echo", restAct)
	s.Restful(2, "PUT", "/api/echo", restAct)
	//HEAD和GET本质一样，区别在于HEAD不含呈现数据，仅仅是HTTP头信息
	s.Restful(2, "HEAD", "/api/echo", restAct)
	s.Restful(2, "DELETE", "/api/echo", restAct)
	s.Restful(2, "OPTIONS", "/api/echo", restAct)
	//传参
	//http://127.0.0.1:8301/full_name/david
	s.Restful(0, "GET", "/full_name/{name}", showFullName)
	//访问设置header content-type:application/json  params:{"name":"jim"}
	s.Restful(0, "PUT", "/full_name", showFullName)
	s.Start1()
}
```
请求例子
```
POST http://127.0.0.1:8301/api/echo HTTP/1.1
Content-Type: application/x-www-form-urlencoded

aaa=12&bbb=hello&ccc=world
```

```
PUT http://127.0.0.1:8301/api/echo HTTP/1.1
Content-Type: application/json

{
    "aaa": 12,
    "bbb": "hello",
    "ccc": "world"
}
```
#### 请求头和响应头

```go
package main

import (
	"github.com/ssgo/s"
	"net/http"
	"os"
)

func headerTest(request *http.Request, response http.ResponseWriter) (token string) {
	token = "Get header token:" + request.Header.Get("token")
	response.Header().Set("resToken", "Hello world")
	return
}

//输入参数放在前放在后都可以
func label(in struct{ Enter string }, 
 request *http.Request, response http.ResponseWriter) (out struct{ Label string}) {
	prefix := request.Header.Get("prefix")
	out.Label = prefix + in.Enter
	response.Header().Set("accept", "application/json")
	return
}

func main() {
	//header
	s.Restful(0, "GET", "/header_test", headerTest)
	s.Restful(0, "POST", "/label", label)
	s.Start1()
}

```
#### 设置响应状态码

使用go标准库自带的response

```go
s.Register(1, "/ssdesign", func(response http.ResponseWriter) string {
	response.WriteHeader(504)
	return "gateway timeout"
})
```

#### 文件上传

文件上传使用标准包自带功能

```go
// 处理/upload 逻辑
func upload(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	file, handler, err := r.FormFile("uploadfile")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()
	fmt.Fprintf(w, "%v", handler.Header)
	f, err := os.OpenFile("./upload/"+handler.Filename, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	io.Copy(f, file)

}
```

#### 过滤器与身份认证

执行先后顺序为：前置过滤器、身份认证、后置过滤器

```go
package main

import (
	"github.com/ssgo/s"
	"net/http"
	"os"
)

type actionFilter struct {
	Aaa     int
	Bbb     string
	Ccc     string
	Filter1 string
	Filter2 int
}

func authTest(in actionFilter) actionFilter {
	return in
}

func main() {
	s.Restful(0, "GET", "/auth_test", authTest)
	s.Restful(1, "POST", "/auth_test", authTest)
	s.Restful(2, "PUT", "/auth_test", authTest)
    //前置过滤器
	s.SetInFilter(func(in *map[string]interface{}, request *http.Request, response *http.ResponseWriter) interface{} {
		(*in)["Filter1"] = "see"
		(*in)["filter2"] = 100
		(*response).Header().Set("content-type", "application/json")
		return nil
	})
    //身份认证
	s.SetAuthChecker(func(authLevel uint, url *string, in *map[string]interface{}, request *http.Request) bool {
		token := request.Header.Get("Token")
		switch authLevel {
		case 1:
			return token == "dev" || token == "develop"
		case 2:
			return token == "dev"
		}
		return false
	})
    //后置过滤器
	s.SetOutFilter(func(in *map[string]interface{}, request *http.Request, response *http.ResponseWriter, result interface{}) (interface{}, bool) {
		data := result.(actionFilter)
		data.Filter2 = data.Filter2 + 100
		return data, false
	})

	s.Start1()
}
```

#### Rewrite

实现对url的重写

```go
func main() {
	s.Register(1, "/show", func(in struct{ S1, S2 string }) string {
		return in.S1 + " " + in.S2
	})
	s.Register(1, "/show/{s1}", func(in struct{ S1, S2 string }) string {
		return in.S1 + " " + in.S2
	})
	s.Rewrite("/r1", "/show")
	//get http://127.0.0.1:8305/r2/123?s2=456 --> http://127.0.0.1:8305/r2/123?s2=456
	s.Rewrite("/r2/(\\w+?)\\?.*?", "/show/$1")
	//post http://127.0.0.1:8305/r3?name=123  s2=456
	s.Rewrite("/r3\\?name=(\\w+)", "/show/$1")
	s.Start1()
}
```
#### 异步服务

启动异步服务与异步服务的调用：

```go
package main

import (
	"fmt"
	"github.com/ssgo/s"
	"net/http"
	"os"
)

type actIn struct {
	Aaa int
	Bbb string
	Ccc string
}

func act(req *http.Request, in actIn) actIn {
	return in
}

func main() {
	s.ResetAllSets()
	//http://127.0.0.1:8301/api/echo?aaa=1&bbb=2&ccc=3
	s.Restful(0, "GET", "/act/echo", act)
	s.Restful(1, "POST", "/act/echo", act)
	//s.Restful(2, "PUT", "/act/echo", act)
	as := s.AsyncStart()
	defer as.Stop()

	asyncPost := as.Post("/act/echo?aaa=hello&bbb=hi", s.Map{
		"ccc": "welcome",
	}, "Cid", "demo-post").Map()
	asyncPut := as.Put("/act/echo", s.Map{
		"aaa": "hello",
		"bbb": "hi",
		"ccc": "welcome",
	}, "Cid", "demo-put").Map()
	asyncGet := as.Get("/act/echo?aaa=11&bbb=222&ccc=333").Map()
	fmt.Println("asyncPut:", asyncPut)
	fmt.Println("asyncPost:", asyncPost)
	fmt.Println("asyncGet", asyncGet)
}

```

#### proxy

将服务代理为自定义服务，支持正则表达式

```go

func main() {
	s.Register(2, "/serv/provide", func() (out struct{ Name string }) {
		out.Name = "server here"
		return
	})
	//调用注册的服务
	s.Register(1, "/serv/gate_get", func(c *discover.Caller) string {
		r := struct{ Name string }{}
		c.Get("a1", "/serv/provide").To(&r)
		return "gate get " + r.Name
	})
	s.Register(1, "/serv/gate_put", func(c *discover.Caller) string {
		r := struct{ Name string }{}
		c.Put("a1", "/serv/provide", "").To(&r)
		return "gate put " + r.Name
	})
	s.Register(2, "/serv/code", func(response http.ResponseWriter) string {
		response.WriteHeader(208)
		return "http code 208"
	})
	s.Proxy("/proxy/test", "a1", "/serv/provide")
	s.Proxy("/proxy/(.+?)", "a1", "/serv/$1") 
	//demo演示，实际场景不推荐这样配置
	os.Setenv("SERVICE_APP", "a1")
	os.Setenv("SERVICE_ACCESSTOKENS", `{"a1_level1": 1, "al_level2": 2}`)
	os.Setenv("SERVICE_CALLS", `{"a1": {"accessToken": "al_level2", "httpVersion": 1}}`)
	base.ResetConfigEnv()
	as := s.AsyncStart1()
	defer as.Stop()

	fmt.Println("/serv/provide", as.Get("/serv/provide", "Access-Token", "accessToken"))
	fmt.Println("/serv/gate_get", as.Get("/serv/gate_get", "Access-Token", "accessToken"))
	fmt.Println("/proxy/test", as.Get("/proxy/test", "Access-Token", "accessToken"))
	fmt.Println("/proxy/gate_put", as.Get("/proxy/gate_put", "Access-Token", "accessToken"))
	fmt.Println("/proxy/code", as.Get("/proxy/code", "Access-Token", "accessToken"))
}
```

#### 静态资源

```go
s.Static("/", "resource")
s.Start()
```
启动服务可以访问站点resource目录下的静态资源

gateway可以通过proxy来实现多个静态服务的负载代理：
```go
s.Proxy("/proxy/(.+?)", "k1", "/$1")
s.Start1()
```

#### Websocket

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

使用websocket：

```go
ws := s.RegisterWebsocket(1, "/dc/ws", updater, open, close, decode, encode)
ws.RegisterAction(0, "hello", func(in struct{ Name string }) 
    (out struct{ Name string }) {
    out.Name = in.Name + "!"
    return
})
c, _, err := websocket.DefaultDialer.Dial("ws://"+addr2+"/proxy/ws", nil)
err = c.WriteJSON(s.Map{"action": "hello", "name": "aaa"})
err = c.ReadJSON(&r)
c.Close()
```

#### cookie

cookie可以使用go标准包http提供的方法，cookie发送给浏览器,即添加一个cookie

```go
func hadler(w http.ResponseWriter) {
	cookieName := http.Cookie{
        Name:     "name",
        Value:    "jim",
        HttpOnly: true,
    }
    cookieToken := http.Cookie{
        Name:       "token",
        Value:      "asd123dsa",
        HttpOnly:   true,
        MaxAge:     60,//设置有效期为60s
    }
    
    w.Header().Set("Set-Cookie", cookieName.String())
    w.Header().Add("Set-Cookie", cookieToken.String())
}
```

使用http的setCookie也可以

```go
func handler2(w http.ResponseWriter) {
    cookieName := http.Cookie{
        Name:     "name",
        Value:    "jim",
        HttpOnly: true,
    }
    cookieToken := http.Cookie{
        Name:     "token",
        Value:    "asd123dsa",
        HttpOnly: true,
    }

    http.SetCookie(w, &cookieName)
    http.SetCookie(w, &cookieToken)
}
```

读取Cookie

```go
func readCookie(w http.ResponseWriter, r *http.Request) {
    cookies := r.Header["Cookie"]
    nameCookie, _ := r.Cookie("name")
}
```

#### SessionKey和SessionInject

```go
// 设置 SessionKey，自动在 Header 中产生，AsyncStart 的客户端支持自动传递
func SetSessionKey(inSessionKey string) {}

// 获取 SessionKey
func GetSessionKey() string {}

// 设置一个生命周期在 Request 中的对象，请求中可以使用对象类型注入参数方便调用
func SetSessionInject(request *http.Request, obj interface{}) {}

// 获取本生命周期中指定类型的 Session 对象
func GetSessionInject(request *http.Request, dataType reflect.Type) interface{} {}
```

基于 Http Header 传递 SessionId（不推荐使用Cookie）

```go
s.Restful(2, "PUT", "/api/echo", action)
s.SetSessionKey("name")
s.Start1()
func showFullName(in struct{ Name string },req *http.Request) (out struct{ FullName string }) {
	out.FullName = in.Name + " Lee." + s.GetSessionId(req)
	return
}
```

使用 SetSession 设置的对象可以在服务方法中直接使用相同类型获得对象，一般是在 AuthChecker 或者 InFilter 中设置

session对象注入

```go
aiValue := actionIn{2, "so", "cool"}
s.SetSessionInject(req, aiValue)
ai := s.GetSessionInject(req, reflect.TypeOf(actionIn{})).(actionIn)
```

#### 对象注入

```go
// 设置一个注入对象，请求中可以使用对象类型注入参数方便调用
func SetInject(obj interface{}) {}

// 获取一个注入对象
func GetInject(dataType reflect.Type) interface{} {}
```

注入对象可以跨请求体

```go
type actionIn struct {
	Aaa int
	Bbb string
	Ccc string
}
func showInject(in struct{ Name string }) (out struct{ FullName string }) {
	ai := s.GetInject(reflect.TypeOf(actionIn{})).(actionIn)
	out.FullName = in.Name + " Lee." + " " + ai.Ccc
	return
}
func main() {
	//……
	aiValue := actionIn{2, "so", "cool"}
	s.SetInject(aiValue)
	//……
}
```

#### 服务调用

服务调用客户端模式

```go

// 调用已注册的服务，根据权重负载均衡
func (caller *Caller) Get(app, path string, headers ... string) *Result {}
func (caller *Caller) Post(app, path string, data interface{}, headers ... string) *Result {}
func (caller *Caller) Put(app, path string, data interface{}, headers ... string) *Result {}
func (caller *Caller) Head(app, path string, data interface{}, headers ... string) *Result {}
func (caller *Caller) Delete(app, path string, data interface{}, headers ... string) *Result {}
func (caller *Caller) Do(app, path string, data interface{}, headers ... string) *Result {}
```

## 负载均衡算法

```go
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