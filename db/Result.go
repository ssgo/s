package db

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
)

type QueryResult struct {
	rows  *sql.Rows
	Sql   *string
	Args  []interface{}
	Error error
}

type ExecResult struct {
	result sql.Result
	Sql    *string
	Args   []interface{}
	Error  error
}

func (r *ExecResult) Changes() int64 {
	if r.result == nil {
		return 0
	}
	numChanges, err := r.result.RowsAffected()
	if err != nil {
		logError(err, r.Sql, r.Args)
		return 0
	}
	return numChanges
}

func (r *ExecResult) Id() int64 {
	if r.result == nil {
		return 0
	}
	insertId, err := r.result.LastInsertId()
	if err != nil {
		logError(err, r.Sql, r.Args)
		return 0
	}
	return insertId
}

func (r *QueryResult) To(result interface{}) error {
	if r.rows == nil {
		return errors.New("operate on a bad query")
	}
	return r.makeResults(result, r.rows)
}

func (r *QueryResult) MapResults() []map[string]interface{} {
	result := make([]map[string]interface{}, 0)
	r.makeResults(&result, r.rows)
	return result
}

func (r *QueryResult) SliceResults() [][]interface{} {
	result := make([][]interface{}, 0)
	r.makeResults(&result, r.rows)
	return result
}

func (r *QueryResult) StringMapResults() []map[string]string {
	result := make([]map[string]string, 0)
	r.makeResults(&result, r.rows)
	return result
}

func (r *QueryResult) StringSliceResults() [][]string {
	result := make([][]string, 0)
	r.makeResults(&result, r.rows)
	return result
}

func (r *QueryResult) MapOnR1() map[string]interface{} {
	result := make(map[string]interface{})
	r.makeResults(&result, r.rows)
	return result
}

func (r *QueryResult) SliceOnR1() []interface{} {
	result := make([]interface{}, 0)
	r.makeResults(&result, r.rows)
	return result
}

func (r *QueryResult) IntsOnC1() []int64 {
	result := make([]int64, 0)
	r.makeResults(&result, r.rows)
	return result
}

func (r *QueryResult) StringsOnC1() []string {
	result := make([]string, 0)
	r.makeResults(&result, r.rows)
	return result
}

func (r *QueryResult) IntOnR1C1() int64 {
	var result int64 = 0
	r.makeResults(&result, r.rows)
	return result
}

func (r *QueryResult) StringOnR1C1() string {
	result := ""
	r.makeResults(&result, r.rows)
	return result
}

func (r *QueryResult) makeResults(results interface{}, rows *sql.Rows) error {
	defer rows.Close()
	rowType := reflect.TypeOf(results)
	resultsValue := reflect.ValueOf(results)
	if rowType.Kind() != reflect.Ptr {
		err := fmt.Errorf("results must be a pointer")
		logError(err, r.Sql, r.Args)
		return err
	}
	rowType = rowType.Elem()
	resultsValue = resultsValue.Elem()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		logError(err, r.Sql, r.Args)
		return err
	}

	colNum := len(colTypes)
	if rowType.Kind() == reflect.Slice {
		// 处理数组类型，非数组类型表示只取一行数据
		rowType = rowType.Elem()
	}

	scanValues := make([]interface{}, colNum)
	if rowType.Kind() == reflect.Struct {
		// 按结构处理数据
		for colIndex, col := range colTypes {
			publicColName := makePublicVarName(col.Name())
			field, found := rowType.FieldByName(publicColName)
			if found {
				if field.Type.Kind() == reflect.Interface {
					scanValues[colIndex] = makeValue(colTypes[colIndex].ScanType())
				} else {
					scanValues[colIndex] = makeValue(field.Type)
				}
			} else {
				scanValues[colIndex] = makeValue(nil)
			}
		}
	} else if rowType.Kind() == reflect.Map {
		// 按Map处理数据
		for colIndex := range colTypes {
			if rowType.Elem().Kind() == reflect.Interface {
				scanValues[colIndex] = makeValue(colTypes[colIndex].ScanType())
			} else {
				scanValues[colIndex] = makeValue(rowType.Elem())
			}
		}
	} else if rowType.Kind() == reflect.Slice {
		// 按Map处理数据
		for colIndex := range colTypes {
			if rowType.Elem().Kind() == reflect.Interface {
				scanValues[colIndex] = makeValue(colTypes[colIndex].ScanType())
			} else {
				scanValues[colIndex] = makeValue(rowType.Elem())
			}
		}
	} else {
		// 只返回一列结果
		if rowType.Kind() == reflect.Interface {
			scanValues[0] = makeValue(colTypes[0].ScanType())
		} else {
			scanValues[0] = makeValue(rowType)
		}
		for colIndex := 1; colIndex < colNum; colIndex++ {
			scanValues[colIndex] = makeValue(nil)
		}
	}

	var data reflect.Value
	for rows.Next() {

		err = rows.Scan(scanValues...)
		if err != nil {
			logError(err, r.Sql, r.Args)
			return err
		}
		if rowType.Kind() == reflect.Struct {
			data = reflect.New(rowType).Elem()
			for colIndex, col := range colTypes {
				publicColName := makePublicVarName(col.Name())
				_, found := rowType.FieldByName(publicColName)
				if found {
					valuePtr := reflect.ValueOf(scanValues[colIndex]).Elem()
					if !valuePtr.IsNil() {
						data.FieldByName(publicColName).Set(valuePtr.Elem())
					}
				}
			}
		} else if rowType.Kind() == reflect.Map {
			// 结果放入Map
			data = reflect.MakeMap(rowType)
			for colIndex, col := range colTypes {
				valuePtr := reflect.ValueOf(scanValues[colIndex]).Elem()
				if !valuePtr.IsNil() {
					data.SetMapIndex(reflect.ValueOf(col.Name()), valuePtr.Elem())
				}
			}
		} else if rowType.Kind() == reflect.Slice {
			// 结果放入Slice
			data = reflect.MakeSlice(rowType, colNum, colNum)
			for colIndex := range colTypes {
				valuePtr := reflect.ValueOf(scanValues[colIndex]).Elem()
				if !valuePtr.IsNil() {
					data.Index(colIndex).Set(valuePtr.Elem())
				}
			}
		} else {
			// 只返回一列结果
			valuePtr := reflect.ValueOf(scanValues[0]).Elem()
			if !valuePtr.IsNil() {
				data = valuePtr.Elem()
			}
		}

		if resultsValue.Kind() == reflect.Slice {
			resultsValue = reflect.Append(resultsValue, data)
		} else {
			resultsValue = data
			break
		}
	}

	reflect.ValueOf(results).Elem().Set(resultsValue)
	//if r.conn.Stats().OpenConnections > 1 {
	//	fmt.Println("conns:", r.conn.Stats().OpenConnections)
	//}
	return nil
}

func makePublicVarName(name string) string {
	colNameBytes := []byte(name)
	if colNameBytes[0] >= 97 {
		colNameBytes[0] -= 32
		return string(colNameBytes)
	} else {
		return name
	}
}

func makeValue(t reflect.Type) interface{} {
	if t == nil {
		return new(*string)
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Int:
		return new(*int)
	case reflect.Int8:
		return new(*int8)
	case reflect.Int16:
		return new(*int16)
	case reflect.Int32:
		return new(*int32)
	case reflect.Int64:
		return new(*int64)
	case reflect.Uint:
		return new(*uint)
	case reflect.Uint8:
		return new(*uint8)
	case reflect.Uint16:
		return new(*uint16)
	case reflect.Uint32:
		return new(*uint32)
	case reflect.Uint64:
		return new(*uint64)
	case reflect.Float32:
		return new(*float32)
	case reflect.Float64:
		return new(*float64)
	case reflect.Bool:
		return new(*bool)
	case reflect.String:
		return new(*string)
	}

	return new(*string)
	//if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8{
	//	return new(string)
	//}
}
