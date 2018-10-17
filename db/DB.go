package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ssgo/s/base"
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

var dbConfigs = make(map[string]*dbInfo)
var dbInstances = make(map[string]*DB)

func GetDB(name string) *DB {
	if dbInstances[name] != nil {
		return dbInstances[name]
	}

	if len(dbConfigs) == 0 {
		base.LoadConfig("db", &dbConfigs)
	}

	conf := dbConfigs[name]
	if conf == nil {
		conf = new(dbInfo)
		dbConfigs[name] = conf
	}
	if conf.Host == "" {
		conf.Host = "127.0.0.1:3306"
	}
	if conf.Type == "" {
		conf.Type = "mysql"
	}
	if conf.User == "" {
		conf.User = "test"
	}
	if conf.DB == "" {
		conf.DB = "test"
	}
	if conf.Password == "" {
		conf.Password = "34RVCy0rQBSQmLX64xjoyg=="
	}

	connectType := "tcp"
	if []byte(conf.Host)[0] == '/' {
		connectType = "unix"
	}

	conn, err := sql.Open(conf.Type, fmt.Sprintf("%s:%s@%s(%s)/%s", conf.User, base.DecryptAes(conf.Password, settedKey, settedIv), connectType, conf.Host, conf.DB))
	if err != nil {
		info := fmt.Sprintf("%s:%s***@%s(%s)/%s", conf.User, conf.Password[0:5], connectType, conf.Host, conf.DB)
		logError(err, &info, nil)
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

func (db *DB) Destroy() error {
	if db.conn == nil {
		return errors.New("operate on a bad connection")
	}
	err := db.conn.Close()
	logError(err, nil, nil)
	return err
}

func (db *DB) GetOriginDB() *sql.DB {
	if db.conn == nil {
		return nil
	}
	return db.conn
}

func (db *DB) Prepare(requestSql string) *Stmt {
	return basePrepare(db.conn, nil, requestSql)
}

func (db *DB) Begin() *Tx {
	if db.conn == nil {
		return &Tx{Error: errors.New("operate on a bad connection")}
	}
	sqlTx, err := db.conn.Begin()
	if err != nil {
		logError(err, nil, nil)
		return &Tx{Error: nil}
	}
	return &Tx{conn: sqlTx}
}

func (db *DB) Exec(requestSql string, args ...interface{}) *ExecResult {
	return baseExec(db.conn, nil, requestSql, args...)
}

func (db *DB) Query(requestSql string, args ...interface{}) *QueryResult {
	return baseQuery(db.conn, nil, requestSql, args...)
}

func (db *DB) Insert(table string, data interface{}) *ExecResult {
	requestSql, values := makeInsertSql(table, data, false)
	return baseExec(db.conn, nil, requestSql, values...)
}
func (db *DB) Replace(table string, data interface{}) *ExecResult {
	requestSql, values := makeInsertSql(table, data, true)
	return baseExec(db.conn, nil, requestSql, values...)
}

func (db *DB) Update(table string, data interface{}, wheres string, args ...interface{}) *ExecResult {
	requestSql, values := makeUpdateSql(table, data, wheres, args...)
	return baseExec(db.conn, nil, requestSql, values...)
}
