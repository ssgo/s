package s

import (
	"bytes"
	"io"
	"path"
	"text/template"
)

//var templates = make(map[string]*template.Template)
//var templatesLock = sync.Mutex{}

func Tpl(data interface{}, functions template.FuncMap, files ...string) string {
	t := getTpl(functions, files...)
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

func TplOut(writer io.Writer, data interface{}, functions template.FuncMap, files ...string) {
	t := getTpl(functions, files...)
	if t != nil {
		err := t.Execute(writer, data)
		if err != nil {
			logError(err.Error(), "tplFile", files)
		}
	}
}

func getTpl(functions template.FuncMap, files ...string) *template.Template {
	//templatesLock.Lock()
	//filesKey := strings.Join(files, ",")
	//t := templates[filesKey]
	//if t == nil {
	tpl := template.New(path.Base(files[0]))
	if functions != nil {
		tpl = tpl.Funcs(functions)
	}
	tt, err := tpl.ParseFiles(files...)
	if err == nil {
		//t = tt
		//templates[filesKey] = t
	} else {
		logError(err.Error(), "tplFile", files)
	}
	//}
	//templatesLock.Unlock()
	//return t

	return tt
}
