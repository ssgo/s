package db

import (
	"github.com/ssgo/base"
	"database/sql"
	"fmt"
	"reflect"
	"time"
	"strings"
	"log"
	"runtime"
)

type dbInfo struct {
	Type        string
	User        string
	Password    string
	Host        string
	DB          string
	MaxOpens    int
	MaxIdles    int
	MaxLifeTime int
}

type DB struct {
	conn  *sql.DB
	Error error
}

type Tx struct {
	conn  *sql.Tx
	Error error
}

type Stmt struct {
	conn  *sql.Stmt
	Error error
}

type QueryResult struct {
	rows  *sql.Rows
	Error error
}

type ExecResult struct {
	result sql.Result
	Error  error
}

var settedKey = []byte("vpL54DlR2KG{JSAaAX7Tu;*#&DnG`M0o")
var settedIv = []byte("@z]zv@10-K.5Al0Dm`@foq9k\"VRfJ^~j")
var keysSetted = false

func SetEncryptKeys(key, iv []byte) {
	if !keysSetted {
		settedKey = key
		settedIv = iv
		keysSetted = true
	}
}

var enabledLogs = true

func EnableLogs(enabled bool) {
	enabledLogs = enabled
}

var dbConfigs = make(map[string]dbInfo)
var dbInstances = make(map[string]*DB)

func GetDB(name string) *DB {
	if dbInstances[name] != nil {
		return dbInstances[name]
	}

	if len(dbConfigs) == 0 {
		base.LoadConfig("db", &dbConfigs)
	}

	conf := dbConfigs[name]
	if conf.Host == "" {
		err := fmt.Errorf("No db seted for %s", name)
		logError(err, 0)
		return &DB{conn: nil, Error: err}
	}
	if conf.Type == "" {
		conf.Type = "mysql"
	}

	connectType := "tcp"
	if []byte(conf.Host)[0] == '/' {
		connectType = "unix"
	}

	conn, err := sql.Open(conf.Type, fmt.Sprintf("%s:%s@%s(%s)/%s", conf.User, base.DecryptAes(conf.Password, settedKey, settedIv), connectType, conf.Host, conf.DB))
	if err != nil {
		logError(err, 0)
		return &DB{conn: nil, Error: err}
	}
	db := new(DB)
	db.conn = conn
	db.Error = nil
	if conf.MaxIdles > 0 {
		conn.SetMaxIdleConns(conf.MaxIdles)
	}
	if conf.MaxOpens > 0 {
		conn.SetMaxOpenConns(conf.MaxOpens)
	}
	if conf.MaxLifeTime > 0 {
		conn.SetConnMaxLifetime(time.Second * time.Duration(conf.MaxLifeTime))
	}
	dbInstances[name] = db
	return db
}

func (this *DB) Destroy() error {
	if this.conn == nil {
		return fmt.Errorf("Operat on a bad connection")
	}
	err := this.conn.Close()
	logError(err, 0)
	return err
}

func (this *DB) GetOriginDB() *sql.DB {
	if this.conn == nil {
		return nil
	}
	return this.conn
}

func (this *Tx) Commit() error {
	if this.conn == nil {
		return fmt.Errorf("Operat on a bad connection")
	}
	err := this.conn.Commit()
	logError(err, 0)
	return err
}

func (this *Tx) Rollback() error {
	if this.conn == nil {
		return fmt.Errorf("Operat on a bad connection")
	}
	err := this.conn.Rollback()
	logError(err, 0)
	return err
}

func (this *Stmt) Exec(args ...interface{}) *ExecResult {
	if this.conn == nil {
		return &ExecResult{Error: fmt.Errorf("Operat on a bad connection")}
	}
	r, err := this.conn.Exec(args...)
	if err != nil {
		logError(err, 0)
		return &ExecResult{Error: err}
	}
	return &ExecResult{result: r}
}

func (this *Stmt) Close() error {
	if this.conn == nil {
		return fmt.Errorf("Operat on a bad connection")
	}
	err := this.conn.Close()
	logError(err, 0)
	return err
}

func (this *DB) Prepare(requestSql string) *Stmt {
	return basePrepare(this.conn, nil, requestSql)
}
func (this *Tx) Prepare(requestSql string) *Stmt {
	return basePrepare(nil, this.conn, requestSql)
}
func basePrepare(db *sql.DB, tx *sql.Tx, requestSql string) *Stmt {
	var sqlStmt *sql.Stmt
	var err error
	if tx != nil {
		sqlStmt, err = tx.Prepare(requestSql)
	} else if db != nil {
		sqlStmt, err = db.Prepare(requestSql)
	} else {
		return &Stmt{Error: fmt.Errorf("Operat on a bad connection")}
	}
	if err != nil {
		logError(err, 1)
		return &Stmt{Error: err}
	}
	return &Stmt{conn: sqlStmt}
}

func (this *DB) Begin() *Tx {
	if this.conn == nil {
		return &Tx{Error: fmt.Errorf("Operat on a bad connection")}
	}
	sqlTx, err := this.conn.Begin()
	if err != nil {
		logError(err, 1)
		return &Tx{Error: nil}
	}
	return &Tx{conn: sqlTx}
}

func (this *DB) Exec(requestSql string, args ...interface{}) *ExecResult {
	return baseExec(this.conn, nil, requestSql, args...)
}
func (this *Tx) Exec(requestSql string, args ...interface{}) *ExecResult {
	return baseExec(nil, this.conn, requestSql, args...)
}
func baseExec(db *sql.DB, tx *sql.Tx, requestSql string, args ...interface{}) *ExecResult {
	var r sql.Result
	var err error
	if tx != nil {
		r, err = tx.Exec(requestSql, args...)
	} else if db != nil {
		r, err = db.Exec(requestSql, args...)
	} else {
		return &ExecResult{Error: fmt.Errorf("Operat on a bad connection")}
	}

	if err != nil {
		logError(err, 1)
		return &ExecResult{Error: err}
	}
	return &ExecResult{result: r}
}

func (this *DB) Query(requestSql string, args ...interface{}) *QueryResult {
	return baseQuery(this.conn, nil, requestSql, args...)
}
func (this *Tx) Query(requestSql string, args ...interface{}) *QueryResult {
	return baseQuery(nil, this.conn, requestSql, args...)
}
func baseQuery(db *sql.DB, tx *sql.Tx, requestSql string, args ...interface{}) *QueryResult {
	var rows *sql.Rows
	var err error
	if tx != nil {
		rows, err = tx.Query(requestSql, args...)
	} else if db != nil {
		rows, err = db.Query(requestSql, args...)
	} else {
		return &QueryResult{Error: fmt.Errorf("Operat on a bad connection")}
	}

	if err != nil {
		logError(err, 1)
		return &QueryResult{Error: err}
	}
	return &QueryResult{rows: rows}
}

func (this *DB) Insert(table string, data interface{}) *ExecResult {
	return baseInsert(this.conn, nil, table, data, false)
}
func (this *Tx) Insert(table string, data interface{}) *ExecResult {
	return baseInsert(nil, this.conn, table, data, false)
}
func (this *DB) Replace(table string, data interface{}) *ExecResult {
	return baseInsert(this.conn, nil, table, data, true)
}
func (this *Tx) Replace(table string, data interface{}) *ExecResult {
	return baseInsert(nil, this.conn, table, data, true)
}
func baseInsert(db *sql.DB, tx *sql.Tx, table string, data interface{}, useReplace bool) *ExecResult {
	keys, vars, values := makeKeysVarsValues(data)
	var operation string
	if useReplace {
		operation = "replace"
	} else {
		operation = "insert"
	}
	requestSql := fmt.Sprintf("%s into `%s` (`%s`) values (%s)", operation, table, strings.Join(keys, "`,`"), strings.Join(vars, ","))

	var result sql.Result
	var err error
	if tx != nil {
		result, err = tx.Exec(requestSql, values...)
	} else if db != nil {
		result, err = db.Exec(requestSql, values...)
	} else {
		return &ExecResult{Error: fmt.Errorf("Operat on a bad connection")}
	}

	if err != nil {
		logError(err, 1)
		return &ExecResult{Error: err}
	}
	return &ExecResult{result: result}
}

func (this *DB) Update(table string, data interface{}, wheres string, args ...interface{}) *ExecResult {
	return baseUpdate(this.conn, nil, table, data, wheres, args...)
}
func (this *Tx) Update(table string, data interface{}, wheres string, args ...interface{}) *ExecResult {
	return baseUpdate(nil, this.conn, table, data, wheres, args...)
}
func baseUpdate(db *sql.DB, tx *sql.Tx, table string, data interface{}, wheres string, args ...interface{}) *ExecResult {
	keys, vars, values := makeKeysVarsValues(data)
	for i, k := range keys {
		keys[i] = fmt.Sprintf("`%s`=%s", k, vars[i])
	}
	for _, v := range args {
		values = append(values, v)
	}
	requestSql := fmt.Sprintf("update `%s` set %s where %s", table, strings.Join(keys, ","), wheres)

	var result sql.Result
	var err error
	if tx != nil {
		result, err = tx.Exec(requestSql, values...)
	} else if db != nil {
		result, err = db.Exec(requestSql, values...)
	} else {
		return &ExecResult{Error: fmt.Errorf("Operat on a bad connection")}
	}
	if err != nil {
		logError(err, 1)
		return &ExecResult{Error: err}
	}
	return &ExecResult{result: result}
}

func (this *ExecResult) Changes() int64 {
	if this.result == nil {
		return 0
	}
	numChanges, err := this.result.RowsAffected()
	if err != nil {
		logError(err, 1)
		return 0
	}
	return numChanges
}

func (this *ExecResult) Id() int64 {
	if this.result == nil {
		return 0
	}
	insertId, err := this.result.LastInsertId()
	if err != nil {
		logError(err, 1)
		return 0
	}
	return insertId
}

func (this *QueryResult) To(result interface{}) error {
	if this.rows == nil {
		return nil
	}
	return makeResults(result, this.rows)
}

func (this *QueryResult) MapResults() []map[string]interface{} {
	result := make([]map[string]interface{}, 0)
	makeResults(&result, this.rows)
	return result
}

func (this *QueryResult) SliceResults() [][]interface{} {
	result := make([][]interface{}, 0)
	makeResults(&result, this.rows)
	return result
}

func (this *QueryResult) StringMapResults() []map[string]string {
	result := make([]map[string]string, 0)
	makeResults(&result, this.rows)
	return result
}

func (this *QueryResult) StringSliceResults() [][]string {
	result := make([][]string, 0)
	makeResults(&result, this.rows)
	return result
}

func (this *QueryResult) MapOnR1() map[string]interface{} {
	result := make(map[string]interface{})
	makeResults(&result, this.rows)
	return result
}

func (this *QueryResult) SliceOnR1() []interface{} {
	result := make([]interface{}, 0)
	makeResults(&result, this.rows)
	return result
}

func (this *QueryResult) IntsOnC1() []int64 {
	result := make([]int64, 0)
	makeResults(&result, this.rows)
	return result
}

func (this *QueryResult) StringsOnC1() []string {
	result := make([]string, 0)
	makeResults(&result, this.rows)
	return result
}

func (this *QueryResult) IntOnR1C1() int64 {
	var result int64 = 0
	makeResults(&result, this.rows)
	return result
}

func (this *QueryResult) StringOnR1C1() string {
	result := ""
	makeResults(&result, this.rows)
	return result
}

func makeKeysVarsValues(data interface{}) ([]string, []string, []interface{}) {
	keys := make([]string, 0)
	vars := make([]string, 0)
	values := make([]interface{}, 0)

	dataType := reflect.TypeOf(data)
	dataValue := reflect.ValueOf(data)
	if dataType.Kind() == reflect.Ptr {
		dataType = dataType.Elem()
		dataValue = dataValue.Elem()
	}

	if dataType.Kind() == reflect.Struct {
		// 按结构处理数据
		for i := 0; i < dataType.NumField(); i++ {
			v := dataValue.Field(i)
			if v.Kind() == reflect.Interface {
				v = v.Elem()
			}
			keys = append(keys, dataType.Field(i).Name)
			if v.Kind() == reflect.String && []byte(v.String())[0] == ':' {
				vars = append(vars, string([]byte(v.String())[1:]))
			} else {
				vars = append(vars, "?")
				values = append(values, v.Interface())
			}
		}
	} else if dataType.Kind() == reflect.Map {
		// 按Map处理数据
		for _, k := range dataValue.MapKeys() {
			v := dataValue.MapIndex(k)
			if v.Kind() == reflect.Interface {
				v = v.Elem()
			}
			keys = append(keys, k.String())
			if v.Kind() == reflect.String && v.Len() > 0 && []byte(v.String())[0] == ':' {
				vars = append(vars, string([]byte(v.String())[1:]))
			} else {
				vars = append(vars, "?")
				values = append(values, v.Interface())
			}
		}
	}

	return keys, vars, values
}

func makeResults(results interface{}, rows *sql.Rows) error {
	rowType := reflect.TypeOf(results)
	resultsValue := reflect.ValueOf(results)
	if rowType.Kind() != reflect.Ptr {
		err := fmt.Errorf("results must be a pointer")
		return err
	}
	rowType = rowType.Elem()
	resultsValue = resultsValue.Elem()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
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
				scanValues[colIndex] = new(string)
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
			scanValues[colIndex] = new(string)
		}
	}

	var data reflect.Value
	for rows.Next() {

		err = rows.Scan(scanValues...)
		if err != nil {
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
		} else {
			resultsValue = data
			break
		}
	}

	reflect.ValueOf(results).Elem().Set(resultsValue)
	//if this.conn.Stats().OpenConnections > 1 {
	//	fmt.Println("conns:", this.conn.Stats().OpenConnections)
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
	if t.Kind() == reflect.Ptr {
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
	//if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8{
	//	return new(string)
	//}
}

func logError(err error, skips int) {
	if enabledLogs && err != nil {
		_, file, lineno, _ := runtime.Caller(skips + 1)
		_, file2, lineno2, _ := runtime.Caller(skips + 2)
		_, file3, lineno3, _ := runtime.Caller(skips + 3)
		_, file4, lineno4, _ := runtime.Caller(skips + 4)
		_, file5, lineno5, _ := runtime.Caller(skips + 5)
		log.Printf("DB	%s	%s:%d	%s:%d	%s:%d	%s:%d	%s:%d", err.Error(), file, lineno, file2, lineno2, file3, lineno3, file4, lineno4, file5, lineno5)
	}
}
