package db

import (
	"database/sql"
	"errors"
)

type Stmt struct {
	conn     *sql.Stmt
	lastSql  *string
	lastArgs []interface{}
	Error    error
}

func (stmt *Stmt) Exec(args ...interface{}) *ExecResult {
	stmt.lastArgs = args
	if stmt.conn == nil {
		return &ExecResult{Sql: stmt.lastSql, Args: stmt.lastArgs, Error: errors.New("operate on a bad connection")}
	}
	r, err := stmt.conn.Exec(args...)
	if err != nil {
		logError(err, stmt.lastSql, stmt.lastArgs)
		return &ExecResult{Sql: stmt.lastSql, Args: stmt.lastArgs, Error: err}
	}
	return &ExecResult{Sql: stmt.lastSql, Args: stmt.lastArgs, result: r}
}

func (stmt *Stmt) Close() error {
	if stmt.conn == nil {
		return errors.New("operate on a bad connection")
	}
	err := stmt.conn.Close()
	logError(err, stmt.lastSql, stmt.lastArgs)
	return err
}
