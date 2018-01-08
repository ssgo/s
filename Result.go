package redis

import (
	"strconv"
	"encoding/json"
	"github.com/ssgo/base"
	"reflect"
)

type Result struct {
	bytesData  []byte
	keys       []string
	bytesDatas [][]byte
	Error      error
}

func toInt64(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		f, err := strconv.ParseFloat(s, 10)
		if err != nil {
			i = 0
		} else {
			i = int64(f)
		}
	}
	return i
}
func toFloat64(s string) float64 {
	f, err := strconv.ParseFloat(s, 10)
	if err != nil {
		f = 0
	}
	return f
}

func (this *Result) Int() int {
	return int(this.Int64())
}
func (this *Result) Uint64() uint64 {
	return uint64(this.Int64())
}
func (this *Result) Uint() uint {
	return uint(this.Int64())
}
func (this *Result) Int64() int64 {
	return toInt64(this.String())
}
func (this *Result) Float() float32 {
	return float32(this.Float64())
}
func (this *Result) Float64() float64 {
	return toFloat64(this.String())
}
func (this *Result) String() string {
	return string(this.bytes())
}
func (this *Result) Bool() bool {
	switch this.String() {
	case "1", "t", "T", "true", "TRUE", "True", "ok", "OK", "Ok", "yes", "YES", "Yes":
		return true
	}
	return false
}
func (this *Result) Ints() []int {
	if this.bytesDatas != nil {
		r := make([]int, len(this.bytesDatas))
		for i, v := range this.bytesDatas {
			r[i] = int(toInt64(string(v)))
		}
		return r
	} else if this.bytesData != nil {
		r := make([]int, 0)
		this.To(&r)
		return r
	}
	return []int{}
}
func (this *Result) Strings() []string {
	if this.bytesDatas != nil {
		r := make([]string, len(this.bytesDatas))
		for i, v := range this.bytesDatas {
			r[i] = string(v)
		}
		return r
	} else if this.bytesData != nil {
		r := make([]string, 0)
		this.To(&r)
		return r
	}
	return []string{}
}
func (this *Result) Results() []Result {
	if this.bytesDatas != nil {
		r := make([]Result, len(this.bytesDatas))
		for i, v := range this.bytesDatas {
			r[i].bytesData = v
		}
		return r
	} else if this.bytesData != nil {
		m := make([]string, 0)
		this.To(&m)
		r := make([]Result, len(m))
		for k, v := range m {
			r[k] = Result{bytesData: []byte(v)}
		}
		return r
	}
	return []Result{}
}
func (this *Result) ResultMap() map[string]*Result {
	if this.bytesDatas != nil && this.keys != nil {
		r := make(map[string]*Result)
		n := len(this.bytesDatas)
		for i, k := range this.keys {
			if i < n {
				r[k] = &Result{bytesData: this.bytesDatas[i]}
			}
		}
		return r
	} else if this.bytesData != nil {
		r := make(map[string]*Result)
		m := make(map[string]string, 0)
		this.To(&m)
		for k, v := range m {
			r[k] = &Result{bytesData: []byte(v)}
		}
		return r
	}
	return map[string]*Result{}
}
func (this *Result) bytes() []byte {
	if this.bytesData != nil {
		return this.bytesData
	}
	return []byte{}
}
func (this *Result) byteSlices() [][]byte {
	if this.bytesDatas == nil {
		return [][]byte{}
	}
	return this.bytesDatas
}
func (this *Result) ToValue(t reflect.Type) reflect.Value {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflect.ValueOf(this.Int64())
	case reflect.Float32, reflect.Float64:
		return reflect.ValueOf(this.Float64())
	case reflect.Bool:
		return reflect.ValueOf(this.Bool())
	case reflect.Map, reflect.Slice, reflect.Struct:
		v := reflect.New(t)
		this.To(v.Interface())
		return v.Elem()
	}
	return reflect.ValueOf(this.String())
}

func (this *Result) To(result interface{}) error {
	var err error = nil
	if this.bytesData != nil {
		base.FixUpperCase(this.bytesData)
		err = json.Unmarshal(this.bytesData, result)
		if err != nil {
			logError(err, 0)
		}
	} else {
		t := reflect.TypeOf(result)
		v := reflect.ValueOf(result)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
			v = v.Elem()
		}
		if (t.Kind() == reflect.Struct || t.Kind() == reflect.Map) && this.keys != nil && this.bytesDatas != nil {
			rs := this.ResultMap()
			for k, r := range rs {
				if t.Kind() == reflect.Struct {
					bytesKey := []byte(k)
					if bytesKey[0] >= 97 {
						bytesKey[0] -= 32
						k = string(bytesKey)
					}
					sf, found := t.FieldByName(k)
					if found {
						v.FieldByName(k).Set(r.ToValue(sf.Type))
					}
				} else if t.Kind() == reflect.Map {
					v.SetMapIndex(reflect.ValueOf(k), r.ToValue(t.Elem()))
				}
			}
		} else if t.Kind() == reflect.Slice && t.Elem().Kind() != reflect.Uint8 && this.bytesDatas != nil {
			rs := this.Results()
			for _, r := range rs {
				v = reflect.Append(v, r.ToValue(t.Elem()))
			}
			reflect.ValueOf(result).Elem().Set(v)
		}

	}
	return err
}
