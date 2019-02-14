# Go的一个服务框架

[![Build Status](https://travis-ci.org/ssgo/s.svg?branch=master)](https://travis-ci.org/ssgo/s)
[![codecov](https://codecov.io/gh/ssgo/s/branch/master/graph/badge.svg)](https://codecov.io/gh/ssgo/s)

ssgo能以非常简单的方式快速部署成为微服务群

## 开始使用

如果您的电脑go version >= 1.11，使用以下命令初始化依赖:

```shell
go mod init sshow
go mod tidy
```

sshow是新建的项目名称

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
export SERVICE_LISTEN=:8080
go run start.go
```

windows下环境变量不区分大小写，windows下使用：

```cmd
set service_listen=:8080
go run start.go
```

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
	// 应用自身自定义key iv
	var settedKey = []byte("vpL54DlR2KG{JSAaAX7Tu;*#&DnG`M0o")
	var settedIv = []byte("@z]zv@10-K.5Al0Dm`@foq9k\"VRfJ^~j")
	redis.SetEncryptKeys(settedKey, settedIv)
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

#### 自定义加密（强烈推荐）

修改AES的秘钥 key 和偏移量 iv 的内容，长度超过32字节

也可以以其他方式只要在 init 函数中调用 redis.SetEncryptKeys 设置匹配的 key和iv即可

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

Register第一个参数值为1表示该服务工作在认证级别1上，派发了一个令牌 “s1token”，不带该令牌的请求将被拒绝

s.Start()将会工作在 HTTP/2.0 No SSL 协议上（服务间通讯默认都使用 HTTP/2.0 No SSL 协议）

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
set service_calls={"s1": {"accessToken": "s1token"}}
go run gateway.go
```

该服务工作在认证级别0上，可以随意访问

s.Start1() 将会工作在 HTTP/1.1 协议上（方便直接测试）

getInfo 方法中调用 s1 时会根据 redis 中注册的节点信息负载均衡到某一个节点

所有调用 s1 服务的请求都会自动带上 "sltoken" 这个令牌以获得相应等级的访问权限


## 框架常用方法

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

// 设置panic错误处理方法
func SetErrorHandle(myErrorHandle func(err interface{}, request *http.Request, response *http.ResponseWriter) interface{})

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

#### https

配置https服务需要在原来配置基础上增加两个环境变量

```shell
export SERVICE_CERTFILE="your cert file path"
export SERVICE_KEYFILE="your key file path"
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

	os.Setenv("SERVICE_LOGFILE", os.DevNull)
	os.Setenv("SERVICE_ACCESSTOKENS", `{"e1_level1": 1, "e1_level2": 2, "e1_level3":3}`)
	os.Setenv("SERVICE_CALLS", `{"e1": {"accessToken": "e1_level3", "httpVersion": 1}}`)
	base.ResetConfigEnv()
	as := s.AsyncStart1()
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
    
    s.SetErrorHandle(func(err interface{}, req *http.Request, rsp *http.ResponseWriter) interface{} {
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
  "callTimeout": 10000,
  "logFile": "",
  "logLevel": "info",
  "noLogGets": false,
  "noLogHeaders": "Accept,Accept-Encoding,Cache-Control,Pragma,Connection",
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
    "news": {
      "accessToken": "hasfjlkdlasfsa",
      "timeout": 5000,
      "httpVersion": 2,
      "withSSL": false
    }
  },
  "callRetryTimes": 10
}
```

| 配置项| 类型 | 样例数据 | 说明 |
|:------ |:------ |:------ |:------ | 
| listen | string | :8081 | 服务绑定的端口号 |
| httpVersion | int | 2 | 服务的http版本 |
| rwTimeout | int<br>毫秒 | 10000 | 服务读写超时时间 |
| keepaliveTimeout | int<br>毫秒 | 10000 | keepalived激活时连接允许空闲的最大时间<br>如果未设置，默认为15秒 |
| callTimeout | int<br>毫秒 | 5000 | 调用服务超时时间 |
| logFile | string | /dev/null | 日志文件<br />设置为nil,不展示日志<br>可以指定日志文件路径<br>不设置默认打向控制台 |
| logLevel | string | info | 指定的日志输出级别<br />debug,info,warning,error |
| httpVersion | int | 2 | 服务的http版本 |
| noLogGets | bool | false | 为true时屏蔽Get网络请求日志 |
| noLogHeaders | string | Accept,Accept-Encoding | 日志请求头和响应头屏蔽header头指定字段输出<br />可设置为false |
| encryptLogFields | string | accessToken | 以***号显示header头敏感信息 <br>默认字段：password,secure,token,accessToken |
| noLogInputFields | string | accessToken | 日志过滤输入的字段，目前未启用<br>为false代表所有字段都日志打印 |
| logInputArrayNum | int | 2 | 输入字段子元素（数组）日志打印个数限制<br>默认为0， s.Arr{1, 2, 3}会输出为float64 (3)|
| logOutputFields | string | code,message | 日志输出的字段白名单<br>默认为false，代表不限制 |
| logOutputArrayNum | int | 3 | 输出字段子元素（数组）日志打印个数限制<br>默认为0 |
| logWebsocketAction | bool | false | 是否展示websocket的WSACTION请求日志 |
| compress | bool | false | 是否开启响应压缩 |
| xUniqueId | string | X-Unique-Id | 请求id |
| xForwardedForName | string | X-Forwarded-For | 请求header头中的X-Forwarded-For HTTP请求端真实IP |
| xRealIpName | string |  X-Real-Ip | http请求机器ip |
| certFile | string |  | https签名证书文件路径 |
| keyFile | string |  | https私钥证书文件路径 |
| registry | string | discover:15 | 服务发现redis的host、密码、数据库、超时时间配置<br>用于服务注册与注销 |
| registryAllowTimeout | string | discover:15:-1 | 服务发现redis允许超时的访问配置<br />用于服务订阅与取消订阅 |
| registryCalls | string | discover:15 | 客户端使用服务发现redis的配置<br />服务节点检查、无效服务节点删除 |
| registryPrefix | string | user- | 服务应用名前缀 |
| app | string | s1 | 可被发现的服务应用名 |
| weight | int | 2 | 负载均衡服务权重 |
| accessTokens | map | {"ad2dc32cde9" : 1} | 服务访问通行码，可以根据不同的授权等级设置多个 |
| appAllows | string |  | 允许访问的服务应用，暂未启用 |
| calls | map |  | 客户端访问服务的配置<br>"s1":{"accessToken": "hasfjlkdlasfsa","timeout": 5000, "httpVersion": 2,"withSSL": false} |
| callRetryTimes | int | 10 | 客户端访问服务失败重试次数 |

#### redis配置

可在应用根目录放置一个 redis.json

服务发现使用的都是discover模块下的配置

```json
{
  "discover": {
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
  "test": {
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

可以在项目根目录放置一个proxy.json来做网关代理配置

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

可以在应用根目录使用env.json综合配置(redis+service+proxy+db)服务：

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

env.json的优先级高于其他配置文件。

如果同级目录下同时出现env和server配置文件，env的配置会对server配置进行覆盖。

#### 环境变量

以下是服务配置

```shell
export SERVICE='{"listen": ":80", "app": "s1"}'
export SERVICE_LISTEN=10.34.22.19:8001
export SERVICE_CALLS_NEWS_ACCESSTOKEN=real_token
```

windows下：

```cmd
set service={"listen": ":80", "app": "s1"}
set service_listen=10.34.22.19:8001
set service_calls_news_accesstoken=real_token
```

以下是服务发现的配置

```shell
export REDIS='{"discover":{"host":"127.0.0.1:6379","db":1}}'
export REDIS_DISCOVER_HOST=127.0.0.1:6379
export REDIS_DISCOVER_PASSWORD='upvNALgTxwS/xUp2Cie4tg=='
```
windows下：

```shell
set redis={"discover":{"host":"127.0.0.1:6379","db":1}}
set redis_discover_host=127.0.0.1:6379
set redis_discover_password=upvNALgTxwS/xUp2Cie4tg==
```

环境变量单项配置优先级大于总体配置

```shell
export SERVICE_CALLS='{"k1": {"accessToken": "s1token"}}'
export SERVICE_CALLS_k1_ACCESSTOKENS=s1-token
```

SERVICE_CALLS_k1_ACCESSTOKENS的配置会覆盖SERVICE_CALLS对k1服务accessToken的配置

#### 配置优先级顺序

cli设置环境变量(set/export) > 配置文件

## 服务调用

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