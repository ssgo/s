package s

import (
	"bytes"
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
			In:        u.If(a.inType != nil, getType(a.inType), ""),
			Out:       u.If(a.funcType.NumOut() > 0, getType(a.funcType.Out(0)), ""),
		}
		out = append(out, api)
	}

	for _, a := range regexWebServices {
		api := Api{
			Type:      "Web",
			Path:      a.path,
			AuthLevel: a.authLevel,
			Method:    a.method,
			In:        u.If(a.inType != nil, getType(a.inType), ""),
			Out:       u.If(a.funcType.NumOut() > 0, getType(a.funcType.Out(0)), ""),
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
			In:        u.If(a.openInType != nil, getType(a.openInType), ""),
			Out:       u.If(a.openFuncType.NumOut() > 0, getType(a.openFuncType.Out(0)), ""),
		}
		out = append(out, api)

		for actionName, action := range a.actions {
			api := Api{
				Type:      "Action",
				Path:      u.StringIf(actionName != "", actionName, "*"),
				AuthLevel: action.authLevel,
				In:        u.If(action.inType != nil, getType(action.inType), ""),
				Out:       u.If(action.funcType.NumOut() > 0, getType(action.funcType.Out(0)), ""),
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
		_, err = fp.Write(data)
		if err != nil {
			log.Error("S", "error", err)
		}
		err = fp.Close()
		if err != nil {
			log.Error("S", "error", err)
		}
	} else {
		log.Error("S", "error", err)
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
					realFromFile = gopath + "/src/github.com/ssgo/" + fromFile
					if fi, err := os.Stat(realFromFile); err != nil || fi == nil {
						log.Error("S", "error", "template file is bad: "+err.Error())
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
		log.Error("S", "error", "template file is bad: "+err.Error())
		return ""
	}

	if toFile == "" {
		buf := bytes.NewBuffer(make([]byte, 0))
		err = t.Execute(buf, data)
		if err != nil {
			log.Error("S", "error", err)
		}
		return buf.String()
	} else {
		fp, err := os.OpenFile(toFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			log.Error("S", "error", "dst file is bad: "+err.Error())
			return ""
		}

		err = t.ExecuteTemplate(fp, fromFile, data)
		if err != nil {
			log.Error("S", "error", err)
		}

		err = fp.Close()
		if err != nil {
			log.Error("S", "error", err)
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
