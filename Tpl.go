package s

import (
	"bytes"
	"html/template"
	"io"
	"sync"
)

var templates = make(map[string]*template.Template)
var templatesLock = sync.Mutex{}

func Tpl(file string, data interface{}) string {
	t := getTpl(file)
	if t != nil {
		buf := bytes.NewBuffer(make([]byte, 0))
		err := t.Execute(buf, data)
		if err != nil {
			logError(err.Error(), "tplFile", file)
		}
		return buf.String()
	}
	return ""
}

func TplOut(file string, writer io.Writer, data interface{}) {
	t := getTpl(file)
	if t != nil {
		err := t.Execute(writer, data)
		if err != nil {
			logError(err.Error(), "tplFile", file)
		}
	}
}

func getTpl(file string) *template.Template {
	templatesLock.Lock()
	t := templates[file]
	//if t == nil {
	tt, err := template.ParseFiles(file)
	if err == nil {
		t = tt
		templates[file] = t
	} else {
		logError(err.Error(), "tplFile", file)
	}
	//}
	templatesLock.Unlock()
	return t
}
