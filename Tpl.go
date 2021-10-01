package s

import (
	"bytes"
	"github.com/ssgo/u"
	"io"
	"path"
	"reflect"
	"strings"
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

func IgnoreTplTags(functions template.FuncMap, tags ...string) template.FuncMap {
	if functions == nil {
		functions = template.FuncMap{}
	}
	for _, tag := range tags {
		functions[tag] = func() string { return "{{" + tag + "}}" }
	}
	return functions
}

func getTpl(functions template.FuncMap, files ...string) *template.Template {
	//templatesLock.Lock()
	//filesKey := strings.Join(files, ",")
	//t := templates[filesKey]
	//if t == nil {
	tpl := template.New(path.Base(files[len(files)-1]))
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

// 为表单添加信息
func ForForm(in interface{}) interface{} {
	r := forForm(reflect.ValueOf(in))
	if r != nil {
		return (*r).Interface()
	}
	return nil
}

func forForm(v1 reflect.Value) *reflect.Value {
	v2 := v1
	v1Type := v1.Type()
	if v1Type.Kind() == reflect.Ptr {
		v2 = v1.Elem()
	}
	v2Type := v2.Type()

	if v2Type.Kind() == reflect.Struct {
		n := v2Type.NumField()
		for i := 0; i < n; i++ {
			v2FieldType := v2Type.Field(i)
			if v2FieldType.Anonymous {
				r := forForm(v2.Field(i))
				if r != nil {
					v2.Field(i).Set(*r)
				}
			} else {
				v2FieldValue := v2.Field(i)
				//fmt.Println("   =>>>", v2FieldType.Name, v2FieldType.Type.Kind(), v2FieldValue.Type().Name())
				if v2FieldType.Type.Kind() == reflect.String && strings.Contains(v2FieldType.Name, "Selected") {
					a := strings.SplitN(v2FieldType.Name, "Selected", 2)
					fromValue := v2.FieldByName(a[0])
					if strings.ContainsRune(a[1], '-') {
						a[1] = strings.ReplaceAll(a[1], "-", "_")
					}
					if fromValue.IsValid() && u.String(u.FinalValue(fromValue).Interface()) == a[1] {
						v2FieldValue.SetString("selected")
					}
					//fmt.Println("   =>>>", v2FieldType.Name, v2FieldValue.Type().Name(), u.String(u.FinalValue(fromValue).Interface()))
				} else if v2FieldType.Type.Kind() == reflect.String && strings.HasSuffix(v2FieldType.Name, "Checked") {
					a := strings.SplitN(v2FieldType.Name, "Checked", 2)
					fromValue := v2.FieldByName(a[0])
					if fromValue.IsValid() && u.Bool(u.FinalValue(fromValue).Interface()) {
						v2FieldValue.SetString("checked")
					}
					//fmt.Println("   =>>>", v2FieldType.Name, v2FieldValue.Type().Name(), fromValue.Interface())
				} else if v2FieldType.Type.Kind() == reflect.Struct {
					r := forForm(v2.Field(i))
					if r != nil {
						v2.Field(i).Set(*r)
					}
				} else if v2FieldType.Type.Kind() == reflect.Slice && v2FieldType.Type.Elem().Kind() == reflect.Struct {
					for j := 0; j < v2FieldValue.Len(); j++ {
						r := forForm(v2FieldValue.Index(j))
						if r != nil {
							v2.Field(i).Index(j).Set(*r)
						}
					}
				} else if v2FieldType.Type.Kind() == reflect.Map && v2FieldType.Type.Elem().Kind() == reflect.Struct {
					for _, k := range v2FieldValue.MapKeys() {
						r := forForm(v2FieldValue.MapIndex(k))
						if r != nil {
							v2.Field(i).SetMapIndex(k, *r)
						}
					}
				}
			}
		}
	}

	if v1Type.Kind() != reflect.Ptr {
		return &v2
	}
	return nil
}
