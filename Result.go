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

func (rs *Result) Int() int {
	return int(rs.Int64())
}
func (rs *Result) Uint64() uint64 {
	return uint64(rs.Int64())
}
func (rs *Result) Uint() uint {
	return uint(rs.Int64())
}
func (rs *Result) Int64() int64 {
	return toInt64(rs.String())
}
func (rs *Result) Float() float32 {
	return float32(rs.Float64())
}
func (rs *Result) Float64() float64 {
	return toFloat64(rs.String())
}
func (rs *Result) String() string {
	return string(rs.bytes())
}
func (rs *Result) Bool() bool {
	switch rs.String() {
	case "1", "t", "T", "true", "TRUE", "True", "ok", "OK", "Ok", "yes", "YES", "Yes":
		return true
	}
	return false
}
func (rs *Result) Ints() []int {
	if rs.bytesDatas != nil {
		r := make([]int, len(rs.bytesDatas))
		for i, v := range rs.bytesDatas {
			r[i] = int(toInt64(string(v)))
		}
		return r
	} else if rs.bytesData != nil {
		r := make([]int, 0)
		rs.To(&r)
		return r
	}
	return []int{}
}
func (rs *Result) Strings() []string {
	if rs.bytesDatas != nil {
		r := make([]string, len(rs.bytesDatas))
		for i, v := range rs.bytesDatas {
			r[i] = string(v)
		}
		return r
	} else if rs.bytesData != nil {
		r := make([]string, 0)
		rs.To(&r)
		return r
	}
	return []string{}
}
func (rs *Result) Results() []Result {
	if rs.bytesDatas != nil {
		r := make([]Result, len(rs.bytesDatas))
		for i, v := range rs.bytesDatas {
			r[i].bytesData = v
		}
		return r
	} else if rs.bytesData != nil {
		m := make([]string, 0)
		rs.To(&m)
		r := make([]Result, len(m))
		for k, v := range m {
			r[k] = Result{bytesData: []byte(v)}
		}
		return r
	}
	return []Result{}
}
func (rs *Result) ResultMap() map[string]*Result {
	if rs.bytesDatas != nil && rs.keys != nil {
		r := make(map[string]*Result)
		n := len(rs.bytesDatas)
		for i, k := range rs.keys {
			if i < n {
				r[k] = &Result{bytesData: rs.bytesDatas[i]}
			}
		}
		return r
	} else if rs.bytesData != nil {
		r := make(map[string]*Result)
		m := make(map[string]string)
		rs.To(&m)
		for k, v := range m {
			r[k] = &Result{bytesData: []byte(v)}
		}
		return r
	}
	return map[string]*Result{}
}
func (rs *Result) StringMap() map[string]string {
	rm := rs.ResultMap()
	m := make(map[string]string)
	for k, r := range rm{
		m[k] = r.String()
	}
	return m
}
func (rs *Result) IntMap() map[string]int {
	rm := rs.ResultMap()
	m := make(map[string]int)
	for k, r := range rm{
		m[k] = r.Int()
	}
	return m
}
func (rs *Result) bytes() []byte {
	if rs.bytesData != nil {
		return rs.bytesData
	}
	return []byte{}
}
func (rs *Result) byteSlices() [][]byte {
	if rs.bytesDatas == nil {
		return [][]byte{}
	}
	return rs.bytesDatas
}
func (rs *Result) ToValue(t reflect.Type) reflect.Value {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflect.ValueOf(rs.Int64())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflect.ValueOf(rs.Uint64())
	case reflect.Float32, reflect.Float64:
		return reflect.ValueOf(rs.Float64())
	case reflect.Bool:
		return reflect.ValueOf(rs.Bool())
	case reflect.Map, reflect.Slice, reflect.Struct:
		v := reflect.New(t)
		rs.To(v.Interface())
		return v.Elem()
	}
	return reflect.ValueOf(rs.String())
}

func (rs *Result) To(result interface{}) error {
	var err error = nil
	if rs.bytesData != nil {
		if len(rs.bytesData) > 0 {
			base.FixUpperCase(rs.bytesData)
			err = json.Unmarshal(rs.bytesData, result)
			if err != nil {
				logError(err, 0)
			}
		}
	} else {
		t := reflect.TypeOf(result)
		v := reflect.ValueOf(result)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
			v = v.Elem()
		}
		if (t.Kind() == reflect.Struct || t.Kind() == reflect.Map) && rs.keys != nil && rs.bytesDatas != nil {
			rs := rs.ResultMap()
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
		} else if t.Kind() == reflect.Slice && t.Elem().Kind() != reflect.Uint8 && rs.bytesDatas != nil {
			rs := rs.Results()
			for _, r := range rs {
				v = reflect.Append(v, r.ToValue(t.Elem()))
			}
			reflect.ValueOf(result).Elem().Set(v)
		}

	}
	return err
}
