package s

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/user"
	"reflect"
	"strings"

	"github.com/ssgo/s/base"
)

type Api struct {
	Type      string
	Path      string
	AuthLevel uint
	Method    string
	In        interface{}
	Out       interface{}
}

// 生成文档数据
func MakeDocument() []Api {
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
			Method:    a.method,
			In:        base.If(a.inType != nil, getType(a.inType), ""),
			Out:       base.If(a.funcType.NumOut() > 0, getType(a.funcType.Out(0)), ""),
		}
		out = append(out, api)
	}

	for _, a := range regexWebServices {
		api := Api{
			Type:      "Web",
			Path:      a.path,
			AuthLevel: a.authLevel,
			Method:    a.method,
			In:        base.If(a.inType != nil, getType(a.inType), ""),
			Out:       base.If(a.funcType.NumOut() > 0, getType(a.funcType.Out(0)), ""),
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
			In:        base.If(a.openInType != nil, getType(a.openInType), ""),
			Out:       base.If(a.openFuncType.NumOut() > 0, getType(a.openFuncType.Out(0)), ""),
		}
		out = append(out, api)

		for actionName, action := range a.actions {
			api := Api{
				Type:      "Action",
				Path:      base.StringIf(actionName != "", actionName, "*"),
				AuthLevel: action.authLevel,
				In:        base.If(action.inType != nil, getType(action.inType), ""),
				Out:       base.If(action.funcType.NumOut() > 0, getType(action.funcType.Out(0)), ""),
			}
			out = append(out, api)
		}
	}
	return out
}

// 生成文档并存储到 json 文件中
func MakeJsonDocumentFile(file string) {
	fp, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	var data []byte
	if err == nil {
		data, err = json.MarshalIndent(MakeDocument(), "", "\t")
	}
	if err == nil {
		fp.Write(data)
		fp.Close()
	} else {
		fmt.Println(err)
	}
}

// 生成文档并存储到 html 文件中，使用默认html模版
func MakeHtmlDocumentFile(title, toFile string) string {
	return MakeHtmlDocumentFromFile(title, toFile, "DocTpl.html")
}

// 生成文档并存储到 html 文件中，使用指定html模版
func MakeHtmlDocumentFromFile(title, toFile, fromFile string) string {
	data := Map{"title": title, "list": MakeDocument()}

	realFromFile := fromFile
	if fi, err := os.Stat(fromFile); err != nil || fi == nil {
		realFromFile = "../" + fromFile
		if fi, err := os.Stat(realFromFile); err != nil || fi == nil {
			realFromFile = os.Args[0][0:strings.LastIndex(os.Args[0], string(os.PathSeparator))] + "/" + fromFile
			if fi, err := os.Stat(realFromFile); err != nil || fi == nil {
				u, _ := user.Current()
				realFromFile = u.HomeDir + "/" + fromFile
				if fi, err := os.Stat(realFromFile); err != nil || fi == nil {
					gopath := os.Getenv("GOPATH")
					if gopath == "" {
						gopath = u.HomeDir + "/go/"
					}
					realFromFile = gopath + "/src/github.com/ssgo/s/" + fromFile
					if fi, err := os.Stat(realFromFile); err != nil || fi == nil {
						Error("S", Map{
							"subLogType": "document",
							"message":    "template file is bad",
							"error":      err.Error(),
						})
						return ""
					}
				}
			}
		}
	}
	t := template.New(title)
	t.Funcs(template.FuncMap{"isMap": isMap, "toText": toText})
	_, err := t.ParseFiles(realFromFile)
	if err != nil {
		Error("S", Map{
			"subLogType": "document",
			"message":    "template file is bad",
			"error":      err.Error(),
		})
		return ""
	}

	if toFile == "" {
		buf := bytes.NewBuffer(make([]byte, 0))
		t.Execute(buf, data)
		return buf.String()
	} else {
		fp, err := os.OpenFile(toFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			Error("S", Map{
				"subLogType": "document",
				"message":    "dst file is bad",
				"error":      err.Error(),
			})
			return ""
		}
		defer fp.Close()

		t.ExecuteTemplate(fp, fromFile, data)
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
