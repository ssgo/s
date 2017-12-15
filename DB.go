package db

import (
	"github.com/ssgo/base"
	"database/sql"
	"fmt"
	"reflect"
	"encoding/base64"
	"crypto/aes"
	"crypto/cipher"
)

type dbInfo struct {
	Type     string
	User     string
	Password string
	Host     string
	DB       string
}

type DB struct {
	conn *sql.DB
}

var key, iv []byte
func SetEncryptKeys(key1, key2 []byte){
	key = key1
	iv = key2
}

func (this *DB) Query(results interface{}, requestSql string, args ...interface{}) error {
	rowType := reflect.TypeOf(results)
	resultsValue := reflect.ValueOf(results)
	if rowType.Kind() != reflect.Ptr {
		return fmt.Errorf("results must be a pointer")
	}
	rowType = rowType.Elem()
	resultsValue = resultsValue.Elem()

	var rows *sql.Rows
	var colTypes []*sql.ColumnType
	var err error

	rows, err = this.conn.Query(requestSql, args...)
	if err == nil {
		colTypes, err = rows.ColumnTypes()
	}
	if err != nil {
		return err
	}

	colNum := len(colTypes)
	if rowType.Kind() == reflect.Slice {
		// 处理数组类型，非数组类型表示只取一行数据
		rowType = rowType.Elem()
	}

	scanValues := make([]interface{},colNum)
	if rowType.Kind() == reflect.Struct {
		// 按结构处理数据
		for colIndex, col := range colTypes {
			publicColName := makePublicVarName(col.Name())
			field, found := rowType.FieldByName(publicColName)
			if found {
				scanValues[colIndex] = makeValue(field.Type)
			}else{
				scanValues[colIndex] = new(string)
			}
		}
	} else if rowType.Kind() == reflect.Map {
		// 按Map处理数据
		for colIndex := range colTypes {
			if rowType.Elem().Kind() == reflect.Interface{
				scanValues[colIndex] = makeValue(colTypes[colIndex].ScanType())
			}else{
				scanValues[colIndex] = makeValue(rowType.Elem())
			}
		}
	} else if rowType.Kind() == reflect.Slice {
		// 按Map处理数据
		for colIndex := range colTypes {
			if rowType.Elem().Kind() == reflect.Interface{
				scanValues[colIndex] = makeValue(colTypes[colIndex].ScanType())
			}else{
				scanValues[colIndex] = makeValue(rowType.Elem())
			}
		}
	} else {
		// 只返回一列结果
		if rowType.Kind() == reflect.Interface{
			scanValues[0] = makeValue(colTypes[0].ScanType())
		}else{
			scanValues[0] = makeValue(rowType)
		}
		for colIndex:=1; colIndex<colNum; colIndex++ {
			scanValues[colIndex] = new(string)
		}
	}

	var data reflect.Value
	for rows.Next() {

		err = rows.Scan(scanValues...)
		if err != nil {
			fmt.Println(err)
			return err
		}
		if rowType.Kind() == reflect.Struct {
			data = reflect.New(rowType).Elem()
			for colIndex, col := range colTypes {
				publicColName := makePublicVarName(col.Name())
				_, found := rowType.FieldByName(publicColName)
				if found {
					data.FieldByName(publicColName).Set(reflect.ValueOf(scanValues[colIndex]).Elem())
				}
			}
		} else if rowType.Kind() == reflect.Map {
			// 结果放入Map
			data = reflect.MakeMap(rowType)
			for colIndex, col := range colTypes {
				data.SetMapIndex(reflect.ValueOf(col.Name()), reflect.ValueOf(scanValues[colIndex]).Elem())
			}
		} else if rowType.Kind() == reflect.Slice {
			// 结果放入Slice
			data = reflect.MakeSlice(rowType, colNum, colNum)
			for colIndex := range colTypes {
				data.Index(colIndex).Set(reflect.ValueOf(scanValues[colIndex]).Elem())
			}
		} else {
			// 只返回一列结果
			data = reflect.ValueOf(scanValues[0]).Elem()
		}

		if resultsValue.Kind() == reflect.Slice {
			resultsValue = reflect.Append(resultsValue, data)
		}else{
			resultsValue = data
			break
		}
	}

	reflect.ValueOf(results).Elem().Set(resultsValue)

	return nil
}

func makePublicVarName(name string) string{
	colNameBytes := []byte(name)
	if colNameBytes[0] >= 97 {
		colNameBytes[0] -= 32
		return string(colNameBytes)
	}else{
		return name
	}
}

func makeValue(t reflect.Type) interface{}{
	if t.Kind() == reflect.Ptr{
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Int:
		return new(int)
	case reflect.Int8:
		return new(int8)
	case reflect.Int16:
		return new(int16)
	case reflect.Int32:
		return new(int32)
	case reflect.Int64:
		return new(int64)
	case reflect.Uint:
		return new(uint)
	case reflect.Uint8:
		return new(uint8)
	case reflect.Uint16:
		return new(uint16)
	case reflect.Uint32:
		return new(uint32)
	case reflect.Uint64:
		return new(uint64)
	case reflect.Float32:
		return new(float32)
	case reflect.Float64:
		return new(float64)
	case reflect.Bool:
		return new(bool)
	case reflect.String:
		return new(string)
	}
	return new(string)
}

func (db *DB) Close() error {
	return db.conn.Close()
}

var dbConfigs = make(map[string]dbInfo)

func GetDB(name string) (*DB, error) {
	if len(dbConfigs) == 0 {
		base.LoadConfig("db", &dbConfigs)
	}

	conf := dbConfigs[name]
	if conf.Host == "" {
		return nil, fmt.Errorf("No db seted for %s", name)
	}
	if conf.Type == "" {
		conf.Type = "mysql"
	}

	conn, err := sql.Open(conf.Type, fmt.Sprintf("%s:%s@tcp(%s)/%s", conf.User, aesDecrypt(conf.Password, key, iv), conf.Host, conf.DB))
	db := new(DB)
	db.conn = conn
	return db, err
}

func aesDecrypt(crypted string, key []byte, iv []byte) string {
	cryptedBytes, err := base64.StdEncoding.DecodeString(crypted)
	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, iv[:blockSize])
	origData := make([]byte, len(cryptedBytes))
	blockMode.CryptBlocks(origData, cryptedBytes)
	origData = pkcs5UnPadding(origData)
	return string(origData)
}

func pkcs5UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}
