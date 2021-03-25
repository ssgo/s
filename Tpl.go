package s

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"text/template"
)

var templates = make(map[string]*template.Template)
var templatesLock = sync.Mutex{}

func Tpl(data interface{}, files ...string) string {
	t := getTpl(files...)
	if t != nil {
		buf := bytes.NewBuffer(make([]byte, 0))
		err := t.Execute(buf, data)
		if err != nil {
			logError(err.Error(), "tplFile", files)
		}
		return buf.String()
	}
	return ""
}

func TplOut(writer io.Writer, data interface{}, files ...string) {
	t := getTpl(files...)
	if t != nil {
		err := t.Execute(writer, data)
		if err != nil {
			logError(err.Error(), "tplFile", files)
		}
	}
}

func getTpl(files ...string) *template.Template {
	templatesLock.Lock()
	filesKey := strings.Join(files, ",")
	t := templates[filesKey]
	//if t == nil {
	tt, err := template.ParseFiles(files...)
	if err == nil {
		t = tt
		templates[filesKey] = t
	} else {
		logError(err.Error(), "tplFile", files)
	}
	//}
	templatesLock.Unlock()
	return t
}
