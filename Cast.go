package base

import (
	"strconv"
	"fmt"
)

func ToInt(value interface{}) int64 {
	switch realValue := value.(type) {
	case int:
	case int8:
	case int16:
	case int32:
	case int64:
	case float32:
	case float64:
		return int64(realValue)
	case bool:
		if realValue{
			return 1
		}else {
			return 0
		}
	case string:
		i, err := strconv.Atoi(realValue)
		if err ==nil {
			return int64(i)
		}
	}
	return 0
}

func ToString(value interface{}) string {
	switch realValue := value.(type) {
	case int:
	case int8:
	case int16:
	case int32:
	case int64:
		return strconv.Itoa(int(realValue))
	case bool:
		if realValue{
			return "true"
		}else {
			return "false"
		}
	case string:
		return realValue
	}
	return fmt.Sprint(value)
}
