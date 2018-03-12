package base

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"reflect"
	"strings"
)

var envConfigs = map[string]string{}
var envUpperConfigs = map[string]string{}
var inited = false

func initConfig() {
	envConf := map[string]interface{}{}
	LoadConfig("env", &envConf)
	initEnvConfigFromFile("", reflect.ValueOf(envConf))
	for _, e := range os.Environ() {
		a := strings.SplitN(e, "=", 2)
		if len(a) == 2 {
			envConfigs[a[0]] = a[1]
		}
	}
	for k1, v1 := range envConfigs {
		envUpperConfigs[strings.ToUpper(k1)] = v1
	}
}

func ResetConfigEnv() {
	envConfigs = map[string]string{}
	envUpperConfigs = map[string]string{}
	initConfig()
}

func LoadConfig(name string, conf interface{}) error {
	if !inited {
		inited = true
		initConfig()
	}

	var file *os.File
	var err error
	file, err = os.Open(name + ".json")
	if err != nil {
		file, err = os.Open("../" + name + ".json")
		if err != nil {
			execPath := os.Args[0][0:strings.LastIndex(os.Args[0], string(os.PathSeparator))]
			file, err = os.Open(execPath + "/" + name + ".json")
			if err != nil {
				u, _ := user.Current()
				if u != nil {
					file, err = os.Open(u.HomeDir + "/" + name + ".json")
				}
			}
		}
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(conf)
	makeEnvConfig(name, reflect.ValueOf(conf))
	return err
}

func makeEnvConfig(prefix string, v reflect.Value) {
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	ev := envConfigs[prefix]
	if ev == "" {
		ev = envUpperConfigs[strings.ToUpper(prefix)]
	}
	if ev != "" {
		if v.CanSet() {
			newValue := reflect.New(t)
			err := json.Unmarshal([]byte(ev), newValue.Interface())
			if err != nil && t.Kind() == reflect.String {
				v.SetString(ev)
			} else if err == nil {
				v.Set(newValue.Elem())
			} else {
				log.Println("LoadConfig", prefix, ev, err)
			}
		} else {
			log.Println("LoadConfig", prefix, ev, "Can't set config because CanSet() == false", t, v)
		}
	}

	if t.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			makeEnvConfig(prefix+"_"+v.Type().Field(i).Name, v.Field(i))
		}
	} else if t.Kind() == reflect.Map {
		// 查找 环境变量 或 env.json 中是否有配置项
		if t.Elem().Kind() != reflect.Interface {
			findPrefix := prefix + "_"
			for k1, _ := range envConfigs {
				if strings.HasPrefix(k1, findPrefix) || strings.HasPrefix(strings.ToUpper(k1), strings.ToUpper(findPrefix)) {
					findPostfix := k1[len(findPrefix):]
					a1 := strings.Split(findPostfix, "_")
					k2 := strings.ToLower(a1[0])
					if k2 != "" && v.MapIndex(reflect.ValueOf(k2)).Kind() == reflect.Invalid {
						var v1 reflect.Value
						if t.Elem().Kind() == reflect.Ptr {
							v1 = reflect.New(t.Elem().Elem())
						} else {
							v1 = reflect.New(t.Elem()).Elem()
						}
						if len(v.MapKeys()) == 0 {
							v.Set(reflect.MakeMap(t))
						}
						v.SetMapIndex(reflect.ValueOf(strings.ToLower(a1[0])), v1)
					}
				}
			}
		}
		for _, mk := range v.MapKeys() {
			//log.Println("	---	", prefix, mk)
			makeEnvConfig(prefix+"_"+mk.String(), v.MapIndex(mk))
		}
	} else if t.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			makeEnvConfig(fmt.Sprint(prefix, "_", i), v.Index(i))
		}
	}
}

func initEnvConfigFromFile(prefix string, v reflect.Value) {
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	if t.Kind() == reflect.Interface {
		t = reflect.TypeOf(v.Interface())
		v = reflect.ValueOf(v.Interface())
	}
	if t.Kind() == reflect.Map {
		if prefix != "" {
			prefix += "_"
		}
		for _, mk := range v.MapKeys() {
			initEnvConfigFromFile(prefix+mk.String(), v.MapIndex(mk))
		}
	} else if t.Kind() == reflect.String {
		envConfigs[prefix] = v.String()
	} else {
		b, err := json.Marshal(v.Interface())
		if err == nil {
			envConfigs[prefix] = string(b)
		} else {
			envConfigs[prefix] = fmt.Sprint(v.Interface())
		}
	}
}
