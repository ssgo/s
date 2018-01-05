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
	conn *sql.DB
}

type Tx struct {
	conn *sql.Tx
}

type Stmt struct {
	conn *sql.Stmt
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

func GetDB(name string) (*DB, error) {
	if dbInstances[name] != nil {
		return dbInstances[name], nil
	}

	if len(dbConfigs) == 0 {
		base.LoadConfig("db", &dbConfigs)
	}

	conf := dbConfigs[name]
	if conf.Host == "" {
		err := fmt.Errorf("No db seted for %s", name)
		logError(err, 0)
		return nil, err
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
		return nil, err
	}
	db := new(DB)
	db.conn = conn
	if conf.MaxIdles > 0 {
		conn.SetMaxIdleConns(conf.MaxIdles)
	}
	if conf.MaxOpens > 0 {
		conn.SetMaxOpenConns(conf.MaxOpens)
	}
	if conf.MaxLifeTime > 0 {
		conn.SetConnMaxLifetime(time.Second*time.Duration(conf.MaxLifeTime))
	}
	dbInstances[name] = db
	return db, err
}

func (this *DB) Destroy() error {
	err := this.conn.Close()
	logError(err, 0)
	return err
}

func (this *DB) GetConnection() *sql.DB {
	return this.conn
}

func (this *Tx) Commit() error {
	err := this.conn.Commit()
	logError(err, 0)
	return err
}

func (this *Tx) Rollback() error {
	err := this.conn.Rollback()
	logError(err, 0)
	return err
}

func (this *Stmt) Exec(args ...interface{}) (int64, error) {
	r, err := this.conn.Exec(args...)
	if err != nil {
		logError(err, 0)
		return 0, err
	}
	numChanges, err := r.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return numChanges, nil
}

func (this *Stmt) Close() error {
	err := this.conn.Close()
	logError(err, 0)
	return err
}

func (this *DB) Prepare(requestSql string) (*Stmt, error) {
	stmt, err := basePrepare(this.conn, nil, requestSql)
	return stmt, err
}
func (this *Tx) Prepare(requestSql string) (*Stmt, error) {
	stmt, err := basePrepare(nil, this.conn, requestSql)
	return stmt, err
}
func basePrepare(db *sql.DB, tx *sql.Tx, requestSql string) (*Stmt, error) {
	var sqlStmt *sql.Stmt
	var err error
	if tx != nil {
		sqlStmt, err = tx.Prepare(requestSql)
	} else {
		sqlStmt, err = db.Prepare(requestSql)
	}
	if err != nil {
		logError(err, 1)
		return nil, err
	}
	stmt := new(Stmt)
	stmt.conn = sqlStmt
	return stmt, nil
}

func (this *DB) Begin() (*Tx, error) {
	sqlTx, err := this.conn.Begin()
	if err != nil {
		logError(err, 1)
		return nil, err
	}
	tx := new(Tx)
	tx.conn = sqlTx
	return tx, nil
}

func (this *DB) Exec(requestSql string, args ...interface{}) (int64, error) {
	return baseExec(this.conn, nil, requestSql, args...)
}
func (this *Tx) Exec(requestSql string, args ...interface{}) (int64, error) {
	return baseExec(nil, this.conn, requestSql, args...)
}
func baseExec(db *sql.DB, tx *sql.Tx, requestSql string, args ...interface{}) (int64, error) {
	var r sql.Result
	var err error
	if tx != nil {
		r, err = tx.Exec(requestSql, args...)
	} else {
		r, err = db.Exec(requestSql, args...)
	}

	if err != nil {
		logError(err, 1)
		return 0, err
	}
	numChanges, err := r.RowsAffected()
	if err != nil {
		logError(err, 1)
		return 0, nil
	}
	return numChanges, nil
}

func (this *DB) ExecInsert(requestSql string, args ...interface{}) (int64, error) {
	return baseExecInsert(this.conn, nil, requestSql, args...)
}
func (this *Tx) ExecInsert(requestSql string, args ...interface{}) (int64, error) {
	return baseExecInsert(nil, this.conn, requestSql, args...)
}
func baseExecInsert(db *sql.DB, tx *sql.Tx, requestSql string, args ...interface{}) (int64, error) {
	var r sql.Result
	var err error
	if tx != nil {
		r, err = tx.Exec(requestSql, args...)
	} else {
		r, err = db.Exec(requestSql, args...)
	}

	if err != nil {
		logError(err, 1)
		return 0, err
	}
	insertId, err := r.LastInsertId()
	if err != nil {
		logError(err, 1)
		return 0, nil
	}
	return insertId, nil
}
func (this *Stmt) ExecInsert(args ...interface{}) (int64, error) {
	r, err := this.conn.Exec(args...)
	if err != nil {
		logError(err, 0)
		return 0, err
	}
	insertId, err := r.LastInsertId()
	if err != nil {
		logError(err, 0)
		return 0, nil
	}
	return insertId, nil
}

func (this *DB) Query(results interface{}, requestSql string, args ...interface{}) error {
	err := baseQuery(this.conn, nil, results, requestSql, args...)
	return err
}
func (this *Tx) Query(results interface{}, requestSql string, args ...interface{}) error {
	err := baseQuery(nil, this.conn, results, requestSql, args...)
	return err
}
func baseQuery(db *sql.DB, tx *sql.Tx, results interface{}, requestSql string, args ...interface{}) error {
	var rows *sql.Rows
	var err error
	if tx != nil {
		rows, err = tx.Query(requestSql, args...)
	} else {
		rows, err = db.Query(requestSql, args...)
	}
	if err != nil {
		logError(err, 1)
		return err
	}
	err = makeResults(results, rows)
	if err != nil {
		logError(err, 1)
		return err
	}
	return err
}

func (this *DB) Insert(table string, data interface{}) (int64, error) {
	return baseInsert(this.conn, nil, table, data, false)
}
func (this *Tx) Insert(table string, data interface{}) (int64, error) {
	return baseInsert(nil, this.conn, table, data, false)
}
func (this *DB) Replace(table string, data interface{}) (int64, error) {
	return baseInsert(this.conn, nil, table, data, true)
}
func (this *Tx) Replace(table string, data interface{}) (int64, error) {
	return baseInsert(nil, this.conn, table, data, true)
}
func baseInsert(db *sql.DB, tx *sql.Tx, table string, data interface{}, useReplace bool) (int64, error) {
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
	} else {
		result, err = db.Exec(requestSql, values...)
	}
	if err != nil {
		logError(err, 1)
		return 0, err
	}
	lastInsertId, err := result.LastInsertId()
	if err != nil {
		logError(err, 1)
		return 0, nil
	}
	return lastInsertId, nil
}

func (this *DB) Update(table string, data interface{}, wheres string, args ...interface{}) (int64, error) {
	return baseUpdate(this.conn, nil, table, data, wheres, args...)
}
func (this *Tx) Update(table string, data interface{}, wheres string, args ...interface{}) (int64, error) {
	return baseUpdate(nil, this.conn, table, data, wheres, args...)
}
func baseUpdate(db *sql.DB, tx *sql.Tx, table string, data interface{}, wheres string, args ...interface{}) (int64, error) {
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
	} else {
		result, err = db.Exec(requestSql, values...)
	}
	if err != nil {
		logError(err, 1)
		return 0, err
	}
	if err != nil {
		logError(err, 1)
		return 0, err
	}
	numChanges, err := result.RowsAffected()
	if err != nil {
		logError(err, 1)
		return 0, nil
	}
	return numChanges, nil
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

func logError (err error, skips int){
	if enabledLogs && err != nil {
		_, file, lineno, _ := runtime.Caller(skips + 1)
		_, file2, lineno2, _ := runtime.Caller(skips + 2)
		_, file3, lineno3, _ := runtime.Caller(skips + 3)
		_, file4, lineno4, _ := runtime.Caller(skips + 4)
		_, file5, lineno5, _ := runtime.Caller(skips + 5)
		log.Printf("DB	%s	%s:%d	%s:%d	%s:%d	%s:%d	%s:%d", err.Error(), file, lineno, file2, lineno2, file3, lineno3, file4, lineno4, file5, lineno5)
	}
}