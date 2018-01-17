package base

import (
	"encoding/json"
	"os"
	"reflect"
	"fmt"
	"strings"
	"log"
	"os/user"
)

func LoadConfig(name string, conf interface{}) error {

	var file *os.File
	var err error
	file, err = os.Open(name + ".json")
	if err != nil {
		execPath := os.Args[0][0:strings.LastIndex(os.Args[0],string(os.PathSeparator))]
		file, err = os.Open(execPath + "/" + name + ".json")
		if err != nil {
			u, _ := user.Current()
			file, err = os.Open(u.HomeDir + "/" + name + ".json")
		}
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(conf)
	makeEnvConfig(strings.ToUpper(name), reflect.ValueOf(conf))
	return err
}

func makeEnvConfig(prefix string, v reflect.Value) {
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()

	ev, has := os.LookupEnv(prefix)
	if has {
		if v.CanSet() {
			newValue := reflect.New(t)
			err := json.Unmarshal([]byte(ev), newValue.Interface())
			if err != nil && t.Kind() == reflect.String {
				v.SetString(ev)
			}else if err == nil {
				v.Set(newValue.Elem())
			} else {
				log.Println("LoadConfig", prefix, ev, err)
			}
			return
		} else {
			log.Println("LoadConfig", prefix, ev, "Can't set config for interface{}")
		}
	}

	if t.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			makeEnvConfig(prefix+"_"+strings.ToUpper(v.Type().Field(i).Name), v.Field(i))
		}
	} else if t.Kind() == reflect.Map {
		for _, mk := range v.MapKeys() {
			makeEnvConfig(prefix+"_"+strings.ToUpper(mk.String()), v.MapIndex(mk))
		}
	} else if t.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			makeEnvConfig(fmt.Sprint(prefix, "_", i), v.Index(i))
		}
	}
}
