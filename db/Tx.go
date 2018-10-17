package db

import (
	"database/sql"
	"errors"
)

type Tx struct {
	conn     *sql.Tx
	lastSql  *string
	lastArgs []interface{}
	Error    error
}

func (tx *Tx) Commit() error {
	if tx.conn == nil {
		return errors.New("operate on a bad connection")
	}
	err := tx.conn.Commit()
	logError(err, tx.lastSql, tx.lastArgs)
	return err
}

func (tx *Tx) Rollback() error {
	if tx.conn == nil {
		return errors.New("operate on a bad connection")
	}
	err := tx.conn.Rollback()
	logError(err, tx.lastSql, tx.lastArgs)
	return err
}

func (tx *Tx) Prepare(requestSql string) *Stmt {
	return basePrepare(nil, tx.conn, requestSql)
}

func (tx *Tx) Exec(requestSql string, args ...interface{}) *ExecResult {
	tx.lastSql = &requestSql
	tx.lastArgs = args
	return baseExec(nil, tx.conn, requestSql, args...)
}

func (tx *Tx) Query(requestSql string, args ...interface{}) *QueryResult {
	tx.lastSql = &requestSql
	tx.lastArgs = args
	return baseQuery(nil, tx.conn, requestSql, args...)
}

func (tx *Tx) Insert(table string, data interface{}) *ExecResult {
	requestSql, values := makeInsertSql(table, data, false)
	tx.lastSql = &requestSql
	tx.lastArgs = values
	return baseExec(nil, tx.conn, requestSql, values...)
}
func (tx *Tx) Replace(table string, data interface{}) *ExecResult {
	requestSql, values := makeInsertSql(table, data, true)
	tx.lastSql = &requestSql
	tx.lastArgs = values
	return baseExec(nil, tx.conn, requestSql, values...)
}

func (tx *Tx) Update(table string, data interface{}, wheres string, args ...interface{}) *ExecResult {
	requestSql, values := makeUpdateSql(table, data, wheres, args...)
	tx.lastSql = &requestSql
	tx.lastArgs = values
	return baseExec(nil, tx.conn, requestSql, values...)
}
