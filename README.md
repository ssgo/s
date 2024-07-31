# Go的一个服务框架

[![Build Status](https://travis-ci.org/ssgo/s.svg?branch=master)](https://travis-ci.org/ssgo/s)
[![codecov](https://codecov.io/gh/ssgo/s/branch/master/graph/badge.svg)](https://codecov.io/gh/ssgo/s)

ssgo能以非常简单的方式快速部署成为微服务群

## 开始使用

如果您的电脑go version >= 1.11，使用以下命令初始化依赖自定义sshow:

```shell
go mod init sshow
go mod tidy
```

1、下载并安装s

```shell
go get -u github.com/ssgo/s
```

2、在项目代码中导入它

```shell
import "github.com/ssgo/s"
```

## 快速构建一个服务

start.go:

```go
package main

import "github.com/ssgo/s"

func main() {
	s.Restful(0, "GET", "/hello", func() string{
		return "Hello ssgo\n"
	})
	s.Start()
}
```

即可快速构建出一个8080端口可访问的服务:

```shell
export service_listen=:8080
export service_httpversion=1
go run start.go
```

windows下使用：

```cmd
set service_listen=:8080
set service_httpversion=1
go run start.go
```

环境变量设置不区分大小写

服务默认使用随机端口启动，若要指定端口可设置环境变量，或start.go目录下配置文件service.json

```json
{
  "listen":":8081"
}
```
开发时可以使用配置文件

部署推荐使用容器技术设置环境变量


## redis

框架服务发现机制基于redis实现，所以使用discover之前，先要准备一个redis服务

默认使用127.0.0.1:6379，db默认为15，密码默认为空，也可以在项目根目录自定义配置redis.json

如果您的redis的密码如果不为空，需要使用AES加密后将密文放在配置文件password字段上，保障密码不泄露

#### 密码使用AES加密
使用github.com/ssgo/tool中的sskey来生成密码：

```shell
cd ssgo/tool/sskey
# 如果以前没有编译过
go build sskey.go
# 创建并保存秘钥
sskey -c sshow
# 输入原始密码
sskey -e sshow
```
得到AES加密后的密码放入discover.json中

```json
{
  "registry":"127.0.0.1:6379:1:upvNALgTxwS/xUp2Cie4tg=="
}
```
也可以通过环境变量来设置：

```shell
export DISCOVER_REGISTRY="127.0.0.1:6379:1:upvNALgTxwS/xUp2Cie4tg=="
```
windows下：
```cmd
set discover_registry="127.0.0.1:6379:1:upvNALgTxwS/xUp2Cie4tg=="
```

discover_registry的设置代表：

redis主机:端口号:数据库:res加密后的密码

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
export discover_app=s1
export service_accesstokens='{"s1token":1}'
go run service.go
```

windows下使用：

```cmd
set discover_app=s1
set service_accesstokens={"s1token":1}
go run service.go
```

Register第一个参数值为1表示该服务工作在认证级别1上，派发了一个令牌 “s1token”，不带该令牌的请求将被拒绝

s.Start()将会工作在 HTTP/2.0 No SSL 协议上（服务间通讯默认都使用 HTTP/2.0 No SSL 协议）

并且自动连接本机默认的redis服务，并注册一个叫 s1 的服务（如需使用其他可以参考redis的配置）

可运行多个实例，调用方访问时将会自动负载均衡到某一个节点

#### Controller

```go
package main

import (
	"github.com/ssgo/s"
	"github.com/ssgo/discover"
)

func getInfo(in struct{ Name string }, c *discover.Caller) (out struct{ FullName string }) {
  c.Get("s1", "/"+in.Name+"/fullName", nil).To(&out)
  return
}

func main() {
  s.Register(0, "/{name}", getInfo)
  s.Start()
}
```

```shell
export discover_app=g1
export service_httpversion=1
export service_listen=:8091
export discover_calls='{"s1":"s1token"}'
go run controller.go &
```


windows下使用：

```cmd
set discover_app=g1
set service_httpversion=1
set service_listen=:8091
set discover_calls={"s1":"s1token"}
go run controller.go
```

该服务工作在认证级别0上，工作在 HTTP/1.1 协议上,可以直接访问

getInfo 方法中调用 s1 时会根据 redis 中注册的节点信息负载均衡到某一个节点

所有调用 s1 服务的请求都会自动带上 "sltoken" 这个令牌以获得相应等级的访问权限

## 框架常用方法

```go
// 注册服务
func Register(authLevel uint, name string, serviceFunc any) {}

// 注册以正则匹配的服务
func RegisterByRegex(name string, service any){}

// 设置前置过滤器
func SetInFilter(filter func(in *map[string]any, request *http.Request, response *http.ResponseWriter) (out any)) {}

// 设置后置过滤器
func SetOutFilter(filter func(in *map[string]any, request *http.Request, response *http.ResponseWriter, out any) (newOut any, isOver bool)) {}

// 注册身份认证模块
func SetAuthChecker(authChecker func(authLevel uint, url *string, request *map[string]any) bool) {}

// 设置panic错误处理方法
func SetErrorHandle(myErrorHandle func(err any, request *http.Request, response *http.ResponseWriter) any)


// 默认启动HTTP/2.0服务（若未配置证书将工作在No SSL模式）
// 如果设置了httpVersion=1则启动HTTP/1.1服务
func Start() {}

// 默认异步方式启动HTTP/2.0服务（）
// 如果设置了httpVersion=1则启动HTTP/1.1服务
func AsyncStart() *AsyncServer {}

// 停止以异步方式启动的服务后等待各种子线程结束
func (as *AsyncServer) Stop() {}

// 调用异步方式启动的服务
func (as *AsyncServer) Get(path string, headers ... string) *Result {}
func (as *AsyncServer) Post(path string, data any, headers ... string) *Result {}
func (as *AsyncServer) Put(path string, data any, headers ... string) *Result {}
func (as *AsyncServer) Head(path string, data any, headers ... string) *Result {}
func (as *AsyncServer) Delete(path string, data any, headers ... string) *Result {}
func (as *AsyncServer) Do(path string, data any, headers ... string) *Result {}

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
	//http://127.0.0.1:8301/api/echo?aaa=1&bbb=2&ccc=3
	s.Restful(0, "GET", "/api/echo", restAct)
	s.Restful(0, "POST", "/api/echo", restAct)
	s.Restful(0, "PUT", "/api/echo", restAct)
	//HEAD和GET本质一样，区别在于HEAD不含呈现数据，仅仅是HTTP头信息
	s.Restful(0, "HEAD", "/api/echo", restAct)
	s.Restful(0, "DELETE", "/api/echo", restAct)
	s.Restful(0, "OPTIONS", "/api/echo", restAct)
	//传参
	//http://127.0.0.1:8301/full_name/david
	s.Restful(0, "GET", "/full_name/{name}", showFullName)
	//访问设置header content-type:application/json  params:{"name":"jim"}
	s.Restful(0, "PUT", "/full_name", showFullName)
	s.Start()
}
```

| 环境变量| 值 |
|:------ |:------ |
| service_listen | :8301 |
| service_httpVersion | 1 |

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

#### https

配置https服务需要在原来配置基础上增加两个环境变量

```shell
export service_certfile="your cert file path"
export service_keyfile="your key file path"
```

windows下：

```shell
set service_certfile=D:/server/ssl/your.pem
set service_keyfile=D:/server/ssl/your.key
```

对于上面的restful实例，如果设置为https服务：

请求例子

```
POST https://127.0.0.1:8301/api/echo HTTP/1.1
Content-Type: application/x-www-form-urlencoded

aaa=12&bbb=hello&ccc=world
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
	s.Start() ////设置service_httpVersion=1
}

```
#### 设置响应状态码

使用go标准库自带的response

```go
s.Register(1, "/ssdesign", func(response http.ResponseWriter) string {
	response.WriteHeader(504)
	return "controller timeout"
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
	s.SetInFilter(func(in *map[string]any, request *http.Request, response *http.ResponseWriter) any {
		(*in)["Filter1"] = "see"
		(*in)["filter2"] = 100
		(*response).Header().Set("content-type", "application/json")
		return nil
	})
    //身份认证
	s.SetAuthChecker(func(authLevel uint, url *string, in *map[string]any, request *http.Request) bool {
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
	s.SetOutFilter(func(in *map[string]any, request *http.Request, response *http.ResponseWriter, result any) (any, bool) {
		data := result.(actionFilter)
		data.Filter2 = data.Filter2 + 100
		return data, false
	})

	s.Start() ////设置service_httpVersion=1
}
```

SetAuthChecker方法return false时，请求会返回403状态码，禁止访问

#### Rewrite

实现对url的重写

```go
func main() {
	s.Register(0, "/show", func(in struct{ S1, S2 string }) string {
		return in.S1 + " " + in.S2
	})
	s.Register(0, "/show/{s1}", func(in struct{ S1, S2 string }) string {
		return in.S1 + " " + in.S2
	})
	s.Rewrite("/r1", "/show")
	//get http://127.0.0.1:8305/r2/123?s2=456 --> http://127.0.0.1:8305/r2/123?s2=456
	s.Rewrite("/r2/(\\w+?)\\?.*?", "/show/$1")
	//post http://127.0.0.1:8305/r3?name=123  s2=456
	s.Rewrite("/r3\\?name=(\\w+)", "/show/$1")
	s.Start() //设置service_httpVersion=1
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
	s.Register(1, "/serv/provide", func() (out struct{ Name string }) {
		out.Name = "server here"
		return
	})
	//调用注册的服务
	s.Register(2, "/serv/gate_get", func(c *discover.Caller) string {
		r := struct{ Name string }{}
		c.Get("e1", "/serv/provide").To(&r)
		return "gate get " + r.Name
	})
	s.Proxy("/proxy/(.+?)", "e1", "/serv/$1")

	os.Setenv("LOG_FILE", os.DevNull)
	os.Setenv("SERVICE_ACCESSTOKENS", `{"e1_level1": 1, "e1_level2": 2, "e1_level3":3}`)
	os.Setenv("DISCOVER_CALLS", `{"e1":"5000:e1_level3:1"}`)
	//一定要做reset，不然手动设置的环境变量不可被加载****
	config.ResetConfigEnv()
	as := s.AsyncStart()
	fmt.Println("/serv/provide:")
	fmt.Println(as.Get("/serv/provide", "Access-Token", "e1_level1"))
	fmt.Println("/serv/gate_get:")
	fmt.Println(as.Get("/serv/gate_get", "Access-Token", "e1_level2"))
	fmt.Println("/proxy/provide:")
	fmt.Println(as.Get("/proxy/provide", "Access-Token", "e1_level2"))
	fmt.Println("/proxy/gate_get:")
	fmt.Println(as.Get("/proxy/gate_get", "Access-Token", "e1_level2"))
	defer as.Stop()
}
```

#### 授权等级

注册服务指定authLevel为0，启动的服务被调用时不需要鉴权，可以直接访问

可以为一个服务指定多个授权等级(1,2,3,4,5,6……)，具备对应授权等级的accessToken的客户端，有权限访问服务

calls客户端使用服务具备的高等级的访问token，可以访问以低授权等级启动的服务

如果accessToken错误，或者具备小于服务启动授权等级的token，访问服务会被拒绝，返回403

例如：

客户端具备6等级accessToken，对于应用下注册启动的1,2,3,4,5,6服务默认具备访问权限

客户端仅具备1等级accessToken，对于应用下注册启动的2,3,4,5,6服务的访问会被拒绝


#### 静态资源

```go
s.Static("/", "resource/")
s.Start()
```

注意：resource结尾一定要有/

启动服务可以访问站点resource目录下的静态资源

controller可以通过proxy来实现多个静态服务的负载代理：
```go
s.Proxy("/proxy/(.+?)", "k1", "/$1")
s.Start()
```


#### Websocket

一个以Action为处理单位的 Websocket 封装

```go
// 注册Websocket服务
func RegisterWebsocket(authLevel uint, name string, updater *websocket.Upgrader,
	onOpen any,
	onClose any,
	decoder func(data any) (action string, request *map[string]any, err error),
	encoder func(action string, data any) any) *ActionRegister {}

// 注册Websocket Action
func (ar *ActionRegister) RegisterAction(authLevel uint, actionName string, action any) {}

// 注册针对 Action 的认证模块
func SetActionAuthChecker(authChecker func(authLevel uint, url *string, action *string, request *map[string]any, sess any) bool) {}

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
func SetSessionInject(request *http.Request, obj any) {}

// 获取本生命周期中指定类型的 Session 对象
func GetSessionInject(request *http.Request, dataType reflect.Type) any {}
```

基于 Http Header 传递 SessionId（不推荐使用Cookie）

```go
s.Restful(2, "PUT", "/api/echo", action)
s.SetSessionKey("name")
s.Start()
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
func SetInject(obj any) {}

// 获取一个注入对象
func GetInject(dataType reflect.Type) any {}
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

#### panic处理

接受服务方法主动panic的处理，可自定义SetErrorHandle

如果没有自定义errorHandle，可以走框架默认的处理方式，建议自定义SetErrorHandle，设置自身的服务方法的统一panic处理

```go
func panicFunc() {
	panic(errors.New("s panic test"))
}

func main() {
    s.ResetAllSets()
    s.Register(0, "/panic_test", panicFunc)
    
    s.SetErrorHandle(func(err any, req *http.Request, rsp *http.ResponseWriter) any {
        return s.Map{"msg": "defined", "code": "30889", "panic": fmt.Sprintf("%s", err)}
    })
    as := s.AsyncStart()
    defer as.Stop()
    
    r := as.Get("/panic_test")
    panicArr := r.Map()
    
    fmt.Println(panicArr)	
}
```

## 配置

#### 服务配置

可在应用根目录放置一个 service.json

```json
{
  "listen": ":8081",
  "httpVersion": 2,
  "rwTimeout": 5000,
  "keepaliveTimeout": 15000,
  "rewriteTimeout": 10000,
  "noLogGets": false,
  "noLogHeaders": "Accept,Accept-Encoding,Cache-Control,Pragma,Connection",
  "noLogInputFields": false,
  "logInputArrayNum": 0,
  "logOutputFields": "code,message",
  "logOutputArrayNum": 2,
  "logWebsocketAction": false,
  "compress": true,
  "certFile": "",
  "keyFile": "",
  "accessTokens": {
    "hasfjlkdlasfsa": 1,
    "fdasfsadfdsa": 2,
    "9ifjjabdsadsa": 2
  }
}
```

| 配置项| 类型 | 样例数据 | 说明 |
|:------ |:------ |:------ |:------ | 
| listen | string | :8081 | 服务绑定的端口号 |
| httpVersion | int | 2 | 服务的http版本 |
| rwTimeout | int<br>毫秒 | 10000 | 服务读写超时时间 |
| keepaliveTimeout | int<br>毫秒 | 10000 | keepalived激活时连接允许空闲的最大时间<br>如果未设置，默认为15秒 |
| rewriteTimeout | int<br>毫秒 | 5000 | rewrite、proxy操作的超时时间 |
| noLogGets | bool | false | 为true时屏蔽Get网络请求日志 |
| noLogHeaders | string | Accept,Accept-Encoding | 日志请求头和响应头屏蔽header头指定字段输出<br />可设置为false |
| noLogInputFields | string | accessToken | 日志过滤输入的字段，目前未启用<br>为false代表所有字段都日志打印 |
| logInputArrayNum | int | 2 | 输入字段子元素（数组）日志打印个数限制<br>默认为0， s.Arr{1, 2, 3}会输出为float64 (3)|
| logOutputFields | string | code,message | 日志输出的字段白名单<br>默认为false，代表不限制 |
| logOutputArrayNum | int | 3 | 输出字段子元素（数组）日志打印个数限制<br>默认为0 |
| logWebsocketAction | bool | false | 是否展示websocket的WSACTION请求日志 |
| compress | bool | false | 是否开启响应gzip压缩(包含静态资源) |
| compressMinSize | int | 1024 | 设置响应内容gzip压缩满足的最小尺寸<br />默认为1024Bytes |
| compressMaxSize | int | 4096000 | 设置响应内容gzip压缩满足的最大尺寸<br />默认为4096000Bytes |
| certFile | string |  | https签名证书文件路径 |
| keyFile | string |  | https私钥证书文件路径 |
| accessTokens | map | {"ad2dc32cde9" : 1} | 当前服务访问授权码，可以根据不同的授权等级设置多个 |
| acceptXRealIpWithoutRequestId| bool | false | 在没有X-Request-ID的情况下是否忽略 X-Real-IP<br />false代表忽略 |

#### 服务发现配置

可在应用根目录放置一个 discover.json

```json
{
  "registry": "127.0.0.1:6379:15",
  "app": "",
  "weight": 1,
  "calls": {
    "s1": "5000:hasfjlkdlasfsa:2:s"
  }
}
```

| 配置项| 类型 | 样例数据 | 说明 |
|:------ |:------ |:------ |:------ | 
| registry | string | 127.0.0.1:6379:15 | 服务发现redis的host、端口、数据库、密码、超时时间配置<br>用于服务注册与注销 |
| app | string | s1 | 可被发现的服务应用名 |
| weight | int | 2 | 负载均衡服务权重 |
| calls | string |  | 客户端访问服务的配置<br>{"s1":"5000:adfad"}<br>{"s1":"5000:s"}<br>{"s1":"adfad:s"}|
| callRetryTimes | int | 10 | 客户端访问服务失败重试次数 |

calls中包含：

timeout:accessToken:httpVersion:SSL

没有固定前后顺序，自动检查配置型

| 配置项| 类型 | 不填默认 | 样例数据 | 说明 |
|:------ |:------ |:------ |:------ |:------ |
| timeout |  int | 5000ms |5000| 如果未设置默认为10000（10秒）,比如：":s1token"   ":s1token:2" | 
| accessToken | string | 空 | 5000 | 比如："5000" | 
| httpVersion | int | http 2.0 no ssl | 2 | 调用服务使用的http协议：1代表http1.1 2代表http2 | 
| SSL | string  | 不使用ssl | s | 是否使用https ssl  这里的值可以不填或者为s|


calls检查顺序：
* 1或2代表http 1.1 2.0  默认为2
* s代表https 没有s 表示不使用ssl
* 字符串代表token
* int类型代表超时的ms
* 其他类型为token



#### 日志配置

可在应用根目录放置一个 log.json
```json
{
  "level": "info",
  "truncations": ["github.com/", "/ssgo/"],
  "sensitive": ["password", "secure", "token", "accessToken"],
  "regexSensitive": ["(^|[^\\d])(1\\d{10})([^\\d]|$)", "\\[(\\w+)\\]"],
  "sensitiveRule": ["11:3*3", "7:2*2", "3:1*1", "2:1*0"]
  
}
```

| 配置项| 类型 | 样例数据 | 说明 |
|:------ |:------ |:------ |:------ | 
| level | string | info | 指定的日志输出级别<br />debug,info,warning,error |
| file | string | /dev/null | 日志文件<br />设置为nil,不展示日志<br>可以指定日志文件路径<br>不设置默认打向控制台 |
| truncations | []string |  ["github.com/", "/ssgo/"] | 程序调用栈callStack字段忽略的目录 |
| sensitive | []string | ["password","token"] | 敏感字段 |
| regexSensitive | []string | ["\\[(\\w+)\\]"] | 日志敏感信息正则匹配 |
| sensitiveRule | []string | ["11:3*3", "7:2*2", "3:1*1", "2:1*0"] | 敏感字段展示规则 |

#### redis配置

可在应用根目录放置一个 redis.json

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
  "dev": {
    "…":"…"
  }
}
```

| 配置项| 类型 | 样例数据 | 说明 |
|:------ |:------ |:------ |:------ | 
| host | string | 127.0.0.1:6379 | host配置 |
| password | string |  | AES加密后的密码 |
| db | int | 1 | 选择的数据库 |
| maxActive | int | 1 | 最大连接数<br>默认为0,代表不限制 |
| maxIdles | int | 10 | 最大空闲连接数，默认为0表示不限制 |
| idleTimeout | int<br>毫秒 | 10000 | keepalived激活时连接允许空闲的最大时间<br>默认为0，代表不限制 |
| connTimeout | int<br>毫秒 | 10000 | 连接超时时间，默认为10s |
| readTimeout | int<br>毫秒 | 10000 | 读超时时间，默认为10s |
| writeTimeout | int<br>毫秒 | 10000 | 写超时时间，默认为10s |

#### 数据库配置

可以在项目根目录放置一个db.json来做mysql数据库的配置

```json
{
  "test": {
    "type": "mysql",
    "user": "test",
    "password": "34RVCy0rQBSQmLX64xjoyg==",	
    "host": "/tmp/mysql.sock",
    "db": "test",
    "maxOpens": 100,	
    "maxIdles": 30,
    "maxLifeTime": 0
  }
}
```

| 配置项| 类型 | 样例数据 | 说明 |
| ------ | ------ | ------ | ------ | 
|type|string|mysql|数据库类型|
|user|string|test|数据库名|
|password|string|  |经过AES加密的数据库密码|
|host|string|  |可为sock文件路径或者mysql的地址、端口号|
|db|string|test|数据库名|
|maxOpens|int| 100 |最大连接数，0表示不限制|
|maxIdles|int| 30 |最大空闲连接，0表示不限制|
|maxLifeTime|int| 0 |每个连接的存活时间，0表示永远|

数据库密码加密可以保障不泄露，和redis加密方法完全相同

也可以以其他方式只要在 init 函数中调用 db.SetEncryptKeys 设置匹配的 key和iv即可

#### 网关代理配置

可以在项目根目录放置一个gateway.json来做网关代理配置

```json
{
  "checkInterval": 1,
  "proxies": {
    "/abc": "k1",
    "/def":"g1",
    "/cce":"g1"
  }
}
```

| 配置项| 类型 | 样例数据 | 说明 |
|:------ |:------ |:------ |:------ | 
|checkInterval|int<br />|10|每隔配置的秒数到redis中获取最新数据<br>最小配置值为3|
|proxies|map|{"/abc": "k1"}|路由到应用名的映射|

##### proxies

proxies可以从环境变量、配置文件、redis中来获取。其中redis配置是动态配置，获取redis中`_proxies`的值，动态更新到gateway应用上。

#### env配置

可以在应用根目录使用env.json综合配置(service+discover+log+gateway+redis+db)服务：

```json
{
  "redis":{
    "test":{
      "host":"127.0.0.1:6379",
      "password":"upvNALgTxwS/xUp2Cie4tg==",
      "db":1
    }
  },
  "service":{
    "listen":":8081"
  },
  "discover": {
    "app":"e1"
  },
  "log": {
    "level": "info"
  },
  "db":{
    "test": {
      "type": "mysql",
      "user": "root",
      "password": "8wv3Kie3Y4nLArmSWs+hng==",
      "host": "127.0.0.1:3306",
      "db": "test"
     }
  }
}
```

<font color=red>env.json的优先级高于其他配置文件。</font>

如果同级目录下同时出现env和server配置文件，env的配置会对server配置进行覆盖。

#### 环境变量

以下是服务配置

```shell
export discover='{"app": "c1", "calls": {"s1":"5000:asfews:1"}}'
export discover_app='c1'
export discover_calls_s1='5000:asfews:2'
```

windows下：

```cmd
set discover={"app": "c1", "calls": {"s1":"5000:asfews"}}
set discover_app=c1
set discover_calls_s1=5000:asfews:2
```

以下是服务发现的redis配置

```shell
export discover='{"REGISTRY":"127.0.0.1:6379:1"}'
export discover_registry='127.0.0.1:6379:1:udigzs+oTp2Kau3Gs20xXQ=='
```
windows下：

```shell
set discover={"registry":"127.0.0.1:6379:1"}
set discover_registry=127.0.0.1:6379:udigzs+oTp2Kau3Gs20xXQ==
```

环境变量单项配置优先级大于总体配置

```shell
export service_calls='{"k1": {"accessToken": "s1token"}}'
export service_calls_k1_accesstokens='s1-token'
```

service_calls_k1_accesstokens的配置会覆盖service_calls对k1服务accessToken的配置

#### 配置优先级顺序

cli设置环境变量(set/export) > 配置文件

## 服务调用

服务调用客户端模式

```go

// 调用已注册的服务，根据权重负载均衡
func (caller *Caller) Get(app, path string, headers ... string) *Result {}
func (caller *Caller) Post(app, path string, data any, headers ... string) *Result {}
func (caller *Caller) Put(app, path string, data any, headers ... string) *Result {}
func (caller *Caller) Head(app, path string, data any, headers ... string) *Result {}
func (caller *Caller) Delete(app, path string, data any, headers ... string) *Result {}
func (caller *Caller) Do(app, path string, data any, headers ... string) *Result {}
```

## 负载均衡算法

```go
// 指定节点调用已注册的服务，并返回本次使用的节点
func (caller *Caller) DoWithNode(method, app, withNode, path string, data any, headers ... string) (*Result, string) {}

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

针对注册好的服务，可轻松实现文档的生成

```go
s.Register(0, "/show1", show1)
s.Register(0, "/show2", show2)
s.Register(0, "/show3", show3)
s.Register(0, "/show4", show4)

s.MakeHtmlDocumentFile("测试文档", "doc.html")
```

生成html文档默认使用s框架根目录下的DocTpl.html作为模板，内部采用`{{ }}`标识语法

DocTpl.html可以作为新建模板的参考

#### 使用命令行创建文档

假设编译好的文件为 ./server

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