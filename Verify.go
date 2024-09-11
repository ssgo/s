package s

import (
	"github.com/ssgo/log"
	"github.com/ssgo/u"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

type VerifyType uint8

const (
	Unknown VerifyType = iota
	Regex
	StringLength
	GreaterThan
	LessThan
	Between
	InList
	ByFunc
)

type VerifySet struct {
	Type       VerifyType
	Regex      *regexp.Regexp
	StringArgs []string
	IntArgs    []int
	FloatArgs  []float64
	Func       func(any, []string) bool
}

var verifySets = map[string]*VerifySet{}
var verifySetsLock = sync.RWMutex{}
var verifyFunctions = map[string]func(any, []string) bool{}

// RegisterVerifyFunc custom a new func verify
func RegisterVerifyFunc(name string, f func(in any, args []string) bool) {
	verifyFunctions[name] = f
}

// RegisterVerify custom a new verify
func RegisterVerify(name, setting string) {
	verifySetsLock.Lock()
	verifySets[name], _ = compileVerifySet(setting)
	verifySetsLock.Unlock()
}

// VerifyStruct verify struct
func VerifyStruct(in any, logger *log.Logger) (ok bool, field string) {
	// 查找最终对象
	var v reflect.Value
	if inValue, succeed := in.(reflect.Value); succeed {
		v = inValue
	} else {
		v = u.FinalValue(reflect.ValueOf(in))
	}
	if v.Kind() != reflect.Struct {
		logger.Error("verify input is not struct", "in", in)
		return false, ""
	}

	// 处理每个字段
	for i := v.NumField() - 1; i >= 0; i-- {
		ft := v.Type().Field(i)
		fv := v.Field(i)
		//fmt.Println("   ====", i, v.NumField(), aa, ft.Name, fv.Kind(), fv)
		if fv.Kind() == reflect.Ptr && (fv.IsNil() || (fv.Elem().Kind() == reflect.String && fv.Elem().String() == "") || (strings.Contains(fv.Elem().Kind().String(), "int") && fv.Elem().Int() == 0)) {
			// 不校验为nil的指针类型
			continue
		}
		if fv.Kind() == reflect.Slice && (fv.IsNil() || fv.Len() == 0) {
			// 不校验空数组
			continue
		}
		if fv.Kind() == reflect.Map && (fv.IsNil() || fv.Len() == 0) {
			// 不校验空对象
			continue
		}
		if ft.Anonymous && fv.CanInterface() {
			// 处理继承
			ok, field = VerifyStruct(fv.Interface(), logger)
			if !ok {
				logger.Warning("verify failed", "in", in, "field", field)
				//fmt.Println("   ====>>1 ", i, v.NumField(), aa, field)
				return false, field
			}
		} else {
			// 处理字段
			tag := ft.Tag.Get("verify")
			keyTag := ft.Tag.Get("verifyKey")
			if len(tag) > 0 || len(keyTag) > 0 {
				// 有效的验证信息
				var err error
				ok, field, err = _verifyValue(fv, tag, keyTag, logger)
				if !ok {
					if field == "" {
						field = u.GetLowerName(ft.Name)
					}
					if err != nil {
						logger.Error(err.Error(), "in", in, "field", field)
					} else {
						logger.Warning("verify failed", "in", in, "field", field)
					}
					//fmt.Println("   ====>>2 ", i, v.NumField(), aa, field)
					return false, field
				}
			}
		}
	}
	return true, ""
}

// 验证数据（反射）
func verifyValue(in reflect.Value, setting string, logger *log.Logger) (bool, string, error) {
	return _verifyValue(in, setting, "", logger)
}

// 验证数据（反射）
func _verifyValue(in reflect.Value, setting string, keySetting string, logger *log.Logger) (bool, string, error) {
	// 验证数组元素
	t := in.Type()
	if t.Kind() == reflect.Slice && t.Elem().Kind() != reflect.Uint8 {
		if len(setting) > 0 {
			for i := 0; i < in.Len(); i++ {
				if ok, field, err := verifyValue(in.Index(i), setting, logger); !ok {
					return false, field, err
				}
			}
		}
		return true, "", nil
	} else if t.Kind() == reflect.Map {
		for _, k := range in.MapKeys() {
			if len(keySetting) > 0 {
				// 验证Key
				if ok, _, err := verifyValue(k, keySetting, logger); !ok {
					return false, "", err
				}
			}
			if len(setting) > 0 {
				if ok, field, err := verifyValue(in.MapIndex(k), setting, logger); !ok {
					return false, field, err
				}
			}
		}
		return true, "", nil
	} else if t.Kind() == reflect.Struct {
		ok, field := VerifyStruct(in, logger)
		return ok, field, nil
	} else {
		if len(setting) == 0 {
			return true, "", nil
		}

		if ok, err := verify(in.Interface(), setting); ok {
			return true, "", nil
		} else {
			return false, "", err
		}
	}
}

// 验证一个数据
func Verify(in any, setting string, logger *log.Logger) (bool, string) {
	ok, field, err := verifyValue(reflect.ValueOf(in), setting, logger)
	if err != nil {
		logger.Error(err.Error(), "in", in, "field", field)
	}
	return ok, field
}

// 验证一个数据
func verify(in any, setting string) (bool, error) {
	if len(setting) < 2 {
		return false, nil
	}
	verifySetsLock.RLock()
	set := verifySets[setting]
	verifySetsLock.RUnlock()
	if set == nil {
		set2, err := compileVerifySet(setting)
		if err != nil {
			return false, err
		}
		set = set2
		verifySetsLock.Lock()
		verifySets[setting] = set
		verifySetsLock.Unlock()
	}

	switch set.Type {
	case ByFunc:
		return set.Func(in, set.StringArgs), nil
	case Regex:
		return set.Regex.MatchString(u.String(in)), nil
	case StringLength:
		if set.StringArgs != nil && set.StringArgs[0] == "+" {
			return len(u.String(in)) >= set.IntArgs[0], nil
		} else if set.StringArgs != nil && set.StringArgs[0] == "-" {
			return len(u.String(in)) <= set.IntArgs[0], nil
		} else if len(set.IntArgs) > 1 {
			l := len(u.String(in))
			return l >= set.IntArgs[0] && l <= set.IntArgs[1], nil
		} else {
			return len(u.String(in)) == set.IntArgs[0], nil
		}
	case GreaterThan:
		return u.Float64(in) > set.FloatArgs[0], nil
	case LessThan:
		return u.Float64(in) < set.FloatArgs[0], nil
	case Between:
		return u.Float64(in) >= set.FloatArgs[0] && u.Float64(in) <= set.FloatArgs[1], nil
	case InList:
		found := false
		inStr := u.String(in)
		for _, item := range set.StringArgs {
			if item == inStr {
				found = true
				break
			}
		}
		return found, nil
	case Unknown:
		return false, nil
	}
	return false, nil
}

// 编译验证设置
func compileVerifySet(setting string) (*VerifySet, error) {
	set := new(VerifySet)
	set.Type = Unknown

	made := false
	if setting[0] != '^' {
		key := setting
		args := ""
		if pos := strings.IndexByte(setting, ':'); pos != -1 {
			key = setting[0:pos]
			args = setting[pos+1:]
		}
		// 查找是否有注册Func
		if verifyFunctions[key] != nil {
			made = true
			set.Type = ByFunc
			if args == "" {
				set.StringArgs = make([]string, 0)
			} else {
				set.StringArgs = strings.Split(args, ",")
			}
			set.Func = verifyFunctions[key]
		}

		// 处理默认支持的类型
		if !made {
			made = true
			switch key {
			case "length":
				// 判断字符串长度
				set.Type = StringLength
				if args == "" {
					args = "1+"
				}
				lastChar := args[len(args)-1]
				if lastChar == '+' || lastChar == '-' {
					set.StringArgs = []string{string(lastChar)}
					args = args[0 : len(args)-1]
				}
				if strings.ContainsRune(args, ',') {
					a := strings.Split(args, ",")
					set.IntArgs = []int{u.Int(a[0]), u.Int(a[1])}
				} else {
					set.IntArgs = []int{u.Int(args)}
				}
			case "between":
				// 判断数字范围
				set.Type = Between
				if args == "" {
					args = "1-100000000"
				}
				a2 := strings.Split(args, "-")
				if len(a2) == 1 {
					// 如果只设置一个参数，范围为1-指定数字
					tempStr := a2[0]
					a2[0] = "0"
					a2 = append(a2, tempStr)
				}
				set.FloatArgs = []float64{u.Float64(a2[0]), u.Float64(a2[1])}
			case "gt":
				// 大于
				set.Type = GreaterThan
				if args == "" {
					args = "0"
				}
				set.FloatArgs = []float64{u.Float64(args)}
			case "lt":
				// 小于
				set.Type = LessThan
				if args == "" {
					args = "0"
				}
				set.FloatArgs = []float64{u.Float64(args)}
			case "in":
				// 枚举
				set.Type = InList
				if args == "" {
					set.StringArgs = make([]string, 0)
				} else {
					set.StringArgs = strings.Split(args, ",")
				}
			default:
				made = false
			}
		}
	}

	if !made {
		rx, err := regexp.Compile(setting)
		if err != nil {
			//log.DefaultLogger.Error(err.Error())
			return nil, err
		} else {
			set.Type = Regex
			set.Regex = rx
		}
	}

	return set, nil
}
