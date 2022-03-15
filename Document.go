package s

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/ssgo/log"
	"html/template"
	"os"
	"os/user"
	"reflect"
	"strings"

	"github.com/ssgo/u"
)

type Api struct {
	Type      string
	Path      string
	AuthLevel int
	Priority  int
	Method    string
	In        interface{}
	Out       interface{}
}

// 生成文档数据
func MakeDocument() ([]Api, []Argot) {
	out := make([]Api, 0)

	for _, a := range rewrites {
		api := Api{
			Type: "Rewrite",
			Path: a.fromPath + " -> " + a.toPath,
		}
		out = append(out, api)
	}

	for _, a := range regexRewrites {
		api := Api{
			Type: "Rewrite",
			Path: a.fromPath + " -> " + a.toPath,
		}
		out = append(out, api)
	}

	for _, a := range proxies {
		api := Api{
			Type: "Proxy",
			Path: a.fromPath + " -> " + a.toApp + ":" + a.toPath,
		}
		out = append(out, api)
	}

	for _, a := range regexProxies {
		api := Api{
			Type: "Proxy",
			Path: a.fromPath + " -> " + a.toApp + ":" + a.toPath,
		}
		out = append(out, api)
	}

	for _, a := range webServices {
		api := Api{
			Type:      "Web",
			Path:      a.path,
			AuthLevel: a.authLevel,
			Priority:  a.options.Priority,
			Method:    a.method,
			In:        "",
			Out:       "",
		}
		if a.inType != nil {
			api.In = getType(a.inType)
		}
		if a.funcType.NumOut() > 0 {
			api.Out = getType(a.funcType.Out(0))
		}
		out = append(out, api)
	}

	for _, a := range regexWebServices {
		api := Api{
			Type:      "Web",
			Path:      a.path,
			AuthLevel: a.authLevel,
			Priority:  a.options.Priority,
			Method:    a.method,
			In:        "",
			Out:       "",
		}

		if a.inType != nil {
			api.In = getType(a.inType)
		}
		if a.funcType.NumOut() > 0 {
			api.Out = getType(a.funcType.Out(0))
		}
		out = append(out, api)
	}

	allWebsocketServices := make([]*websocketServiceType, 0)
	for _, a := range websocketServices {
		allWebsocketServices = append(allWebsocketServices, a)
	}
	for _, a := range regexWebsocketServices {
		allWebsocketServices = append(allWebsocketServices, a)
	}
	for _, a := range allWebsocketServices {
		api := Api{
			Type:      "WebSocket",
			Path:      a.path,
			AuthLevel: a.authLevel,
			Priority:  a.options.Priority,
			In:        "",
			Out:       "",
		}
		if a.openInType != nil {
			api.In = getType(a.openInType)
		}
		if a.openFuncType.NumOut() > 0 {
			api.Out = getType(a.openFuncType.Out(0))
		}
		out = append(out, api)

		for actionName, action := range a.actions {
			api := Api{
				Type:      "Action",
				Path:      u.StringIf(actionName != "", actionName, "*"),
				AuthLevel: action.authLevel,
				Priority:  action.priority,
				In:        "",
				Out:       "",
			}
			if action.inType != nil {
				api.In = getType(action.inType)
			}
			if action.funcType.NumOut() > 0 {
				api.Out = getType(action.funcType.Out(0))
			}
			out = append(out, api)
		}
	}
	return out, _argots
}

// 生成文档并存储到 json 文件中
func MakeJsonDocument() string {
	api, argots := MakeDocument()
	data, err := json.Marshal(map[string]interface{}{
		"api":    api,
		"argots": argots,
	})

	u.FixUpperCase(data, nil)
	api2 := Map{}
	json.Unmarshal(data, &api2)
	data2, _ := json.MarshalIndent(api2, "", "\t")

	if err == nil {
		return string(data2)
	}
	return ""
}

func MakeJsonDocumentFile(file string) {
	fp, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	jsonData := MakeJsonDocument()
	if err == nil && jsonData != "" {
		_, err = fp.Write([]byte(jsonData))
		if err != nil {
			log.DefaultLogger.Error(err.Error())
		}
		err = fp.Close()
		if err != nil {
			log.DefaultLogger.Error(err.Error())
		}
	} else {
		log.DefaultLogger.Error(err.Error())
	}
}

// 生成文档并存储到 html 文件中，使用默认html模版
func MakeHtmlDocumentFile(title, toFile string) string {
	return MakeHtmlDocumentFromFile(title, toFile, "DocTpl.html")
}

//go:embed DocTpl.html
var defaultDocTpl string

// 生成文档并存储到 html 文件中，使用指定html模版
func MakeHtmlDocumentFromFile(title, toFile, fromFile string) string {
	api, argots := MakeDocument()

	for i, a := range api {
		data2, _ := json.Marshal(a.In)
		u.FixUpperCase(data2, nil)
		var in2 interface{}
		_ = json.Unmarshal(data2, &in2)
		api[i].In = in2

		if out4, ok := a.Out.(Map); ok {
			//fmt.Println("============", out4)
			if out4["Result"] != nil {
				result4 := out4["Result"].(Map)
				for k, v := range result4 {
					out4[k] = v
				}
				delete(out4, "Result")
			}
			a.Out = out4
		} else {
			//fmt.Println(">>>>>>>", a.Out)
		}
		data3, _ := json.Marshal(a.Out)
		u.FixUpperCase(data3, nil)
		var out3 interface{}
		_ = json.Unmarshal(data3, &out3)
		api[i].Out = out3
	}

	data := Map{"title": title, "api": api, "argots": argots}

	realFromFile := fromFile
	if fi, err := os.Stat(fromFile); err != nil || fi == nil {
		realFromFile = "../" + fromFile
		if fi, err := os.Stat(realFromFile); err != nil || fi == nil {
			realFromFile = os.Args[0][0:strings.LastIndex(os.Args[0], string(os.PathSeparator))] + "/" + fromFile
			if fi, err := os.Stat(realFromFile); err != nil || fi == nil {
				currentUser, _ := user.Current()
				realFromFile = currentUser.HomeDir + "/" + fromFile
				if fi, err := os.Stat(realFromFile); err != nil || fi == nil {
					gopath := os.Getenv("GOPATH")
					if gopath == "" {
						gopath = currentUser.HomeDir + "/go/"
					}
					realFromFile = gopath + "/src/github.com/ssgo/" + fromFile
					if fi, err := os.Stat(realFromFile); err != nil || fi == nil {
						realFromFile = ""
						//log.DefaultLogger.Error("template file is bad: " + err.Error())
						//return ""
					}
				}
			}
		}
	}

	t := template.New(title)
	t.Funcs(template.FuncMap{"isMap": isMap, "toText": toText})
	var err error
	if realFromFile != "" {
		_, err = t.ParseFiles(realFromFile)
	} else {
		_, err = t.Parse(defaultDocTpl)
	}
	if err != nil {
		log.DefaultLogger.Error("template file is bad: " + err.Error())
		return ""
	}

	if toFile == "" {
		buf := bytes.NewBuffer(make([]byte, 0))
		err = t.Execute(buf, data)
		if err != nil {
			log.DefaultLogger.Error(err.Error())
		}
		return buf.String()
	} else {
		fp, err := os.OpenFile(toFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			log.DefaultLogger.Error("dst file is bad: " + err.Error())
			return ""
		}

		err = t.ExecuteTemplate(fp, fromFile, data)
		if err != nil {
			log.DefaultLogger.Error(err.Error())
		}

		err = fp.Close()
		if err != nil {
			log.DefaultLogger.Error(err.Error())
		}
		return ""
	}
}

func getType(t reflect.Type) interface{} {
	if t == nil {
		return ""
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		outs := Map{}
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).Tag != "" && reflect.ValueOf(outs[t.Field(i).Name]).Kind() == reflect.String {
				outs[t.Field(i).Name] = fmt.Sprint(outs[t.Field(i).Name].(string), " ", t.Field(i).Tag)
			} else {
				outs[t.Field(i).Name] = getType(t.Field(i).Type)
			}
		}
		return outs
	case reflect.Map:
		return fmt.Sprintf("map[%s]%s", getType(t.Key()), getType(t.Elem()))
	case reflect.Slice:
		return fmt.Sprint("[]", getType(t.Elem()))
	case reflect.Interface:
		return "*"
	default:
		return t.String()
	}
}

func isMap(arg interface{}) bool {
	if arg == nil {
		return false
	}
	t := reflect.TypeOf(arg)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	argKind := t.Kind()
	return argKind == reflect.Map || argKind == reflect.Struct
}

func toText(arg interface{}) string {
	t := reflect.TypeOf(arg)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.String {
		return arg.(string)
	}
	argBytes, _ := json.MarshalIndent(arg, "", "\t")
	return string(argBytes)
}
