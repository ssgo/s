package tests

import _ "github.com/go-sql-driver/mysql"
import (
	".."
	"testing"
	"strings"
	"time"
)

type userInfo struct {
	Id    int
	Name  string
	Phone string
	Time  string
}

func TestBaseSelect(t *testing.T) {

	sql := "SELECT 1002 id, '13800000001' phone"
	db, err := db.GetDB("test")
	if err != nil {
		t.Error("GetDB error", err)
		return
	}

	results1 := make([]map[string]interface{}, 0)
	err = db.Query(&results1, sql)
	if err != nil {
		t.Error("Query error", sql, results1, err)
	} else if results1[0]["id"].(int64) != 1002 || results1[0]["phone"].(string) != "13800000001" {
		t.Error("Result error", sql, results1, err)
	}

	results2 := make([]map[string]string, 0)
	err = db.Query(&results2, sql)
	if err != nil {
		t.Error("Query error", sql, results2, err)
	} else if results2[0]["id"] != "1002" || results2[0]["phone"] != "13800000001" {
		t.Error("Result error", sql, results2, err)
	}

	results3 := make([]map[string]int, 0)
	err = db.Query(&results3, sql)
	if err != nil {
		t.Error("Query error", sql, results3, err)
	} else if results3[0]["id"] != 1002 || results3[0]["phone"] != 13800000001 {
		t.Error("Result error", sql, results3, err)
	}

	results4 := make([]userInfo, 0)
	err = db.Query(&results4, sql)
	if err != nil {
		t.Error("Query error", sql, results4, err)
	} else if results4[0].Id != 1002 || results4[0].Phone != "13800000001" {
		t.Error("Result error", sql, results4, err)
	}

	results5 := make([][]string, 0)
	err = db.Query(&results5, sql)
	if err != nil {
		t.Error("Query error", sql, results5, err)
	} else if results5[0][0] != "1002" || results5[0][1] != "13800000001" {
		t.Error("Result error", sql, results5, err)
	}

	results6 := make([]string, 0)
	err = db.Query(&results6, sql)
	if err != nil {
		t.Error("Query error", sql, results6, err)
	} else if results6[0] != "1002" {
		t.Error("Result error", sql, results6, err)
	}

	results7 := map[string]interface{}{}
	err = db.Query(&results7, sql)
	if err != nil {
		t.Error("Query error", sql, results7, err)
	} else if results7["id"].(int64) != 1002 || results7["phone"] != "13800000001" {
		t.Error("Result error", sql, results7, err)
	}

	results8 := userInfo{}
	err = db.Query(&results8, sql)
	if err != nil {
		t.Error("Query error", sql, results8, err)
	} else if results8.Id != 1002 || results8.Phone != "13800000001" {
		t.Error("Result error", sql, results8, err)
	}

	var results9 int
	err = db.Query(&results9, sql)
	if err != nil {
		t.Error("Query error", sql, results9, err)
	} else if results9 != 1002 {
		t.Error("Result error", sql, results9, err)
	}

	t.Log("OpenConnections", db.GetConnection().Stats().OpenConnections)
}

func TestInsertReplaceUpdateDelete(t *testing.T) {
	db := initDB(t)
	insertId, err := db.Insert("tempUsersForDBTest", map[string]interface{}{
		"phone": 18033336666,
		"name":  "Star",
		"time": ":DATE_SUB(NOW(), INTERVAL 1 DAY)",
	})
	if err != nil {
		t.Error("Insert 1 error", err)
	}
	if insertId != 1 {
		t.Error("insertId 1 error", insertId, err)
	}

	insertId, err = db.Insert("tempUsersForDBTest", map[string]interface{}{
		"phone": "18000000002",
		"name":  "Tom",
	})
	if err != nil {
		t.Error("Insert 2 error", err)
	}
	if insertId != 2 {
		t.Error("insertId 2 error", insertId, err)
	}

	numChanges, err := db.Update("tempUsersForDBTest", map[string]interface{}{
		"phone": "18000000222",
		"name":  "Tom Lee",
	}, "id=?", 2)
	if err != nil {
		t.Error("Update 2 error", err)
	}
	if numChanges != 1 {
		t.Error("Update 2 num error", numChanges, err)
	}

	insertId, err = db.Replace("tempUsersForDBTest", map[string]interface{}{
		"phone": "18000000003",
		"name":  "Amy",
	})
	if err != nil {
		t.Error("Replace 3 error", err)
	}
	if insertId != 3 {
		t.Error("insertId 3 error", insertId, err)
	}

	numChanges, err = db.Exec("delete from tempUsersForDBTest where id=3")
	if err != nil {
		t.Error("Delete 3 error", err)
	}
	if numChanges != 1 {
		t.Error("Delete 3 num error", numChanges, err)
	}

	insertId, err = db.Replace("tempUsersForDBTest", map[string]interface{}{
		"phone": "18000000004",
		"name":  "Jerry",
	})
	if err != nil {
		t.Error("Replace 4 error", err)
	}
	if insertId != 4 {
		t.Error("insertId 4 error", insertId, err)
	}

	stmt, err := db.Prepare("replace into `tempUsersForDBTest` (`id`,`phone`,`name`) values (?,?,?)")
	if err != nil {
		t.Error("Prepare 4 error", err)
	}
	insertId, err = stmt.ExecInsert(4, "18000000004", "Jerry's Mather")
	stmt.Close()

	if err != nil {
		t.Error("Replace 4 error", err)
	}
	if insertId != 4 {
		t.Error("insertId 4 error", insertId, err)
	}

	userList := make([]userInfo, 0)
	err = db.Query(&userList, "select * from tempUsersForDBTest")
	if err != nil {
		t.Error("Select userList error", err)
	}
	if strings.Split(userList[0].Time, " ")[0] != time.Now().Add(time.Hour*24*-1).Format("2006-01-02") || userList[0].Id != 1 || userList[0].Name != "Star" || userList[0].Phone != "18033336666" {
		t.Error("Select userList 1 error", userList, err)
	}
	if strings.Split(userList[1].Time, " ")[0] != time.Now().Format("2006-01-02") || userList[1].Id != 2 || userList[1].Name != "Tom Lee" || userList[1].Phone != "18000000222" {
		t.Error("Select userList 1 error", userList, err)
	}
	if userList[2].Id != 4 || userList[2].Name != "Jerry's Mather" || userList[2].Phone != "18000000004" {
		t.Error("Select userList 1 error", userList, err)
	}

	finishDB(db, t)
}

func TestTransaction(t *testing.T) {
	var userList []userInfo

	db := initDB(t)
	tx, err := db.Begin()
	if err != nil {
		t.Error("Begin error", err)
	}

	tx.Insert("tempUsersForDBTest", map[string]interface{}{
		"phone": 18033336666,
		"name":  "Star",
		"time": ":DATE_SUB(NOW(), INTERVAL 1 DAY)",
	})

	userList = make([]userInfo, 0)
	err = db.Query(&userList, "select * from tempUsersForDBTest")
	if err != nil || len(userList) != 0 {
		t.Error("Select Out Of TX", userList, err)
	}

	userList = make([]userInfo, 0)
	err = tx.Query(&userList, "select * from tempUsersForDBTest")
	if err != nil || len(userList) != 1 {
		t.Error("Select In TX", userList, err)
	}

	tx.Rollback()

	userList = make([]userInfo, 0)
	err = db.Query(&userList, "select * from tempUsersForDBTest")
	if err != nil || len(userList) != 0 {
		t.Error("Select When Rollback", userList, err)
	}

	tx, err = db.Begin()
	if err != nil {
		t.Error("Begin 2 error", err)
	}

	stmt, err := tx.Prepare("insert into `tempUsersForDBTest` (`id`,`phone`,`name`) values (?,?,?)")
	if err != nil {
		t.Error("Prepare 4 error", err)
	}
	stmt.ExecInsert(4, "18000000004", "Jerry's Mather")
	stmt.Close()

	tx.Commit()

	userList = make([]userInfo, 0)
	err = db.Query(&userList, "select * from tempUsersForDBTest")
	if err != nil || len(userList) != 1 {
		t.Error("Select When Commit", userList, err)
	}

	finishDB(db, t)
}

func initDB(t *testing.T) *db.DB {
	db, err := db.GetDB("test")
	if err != nil {
		t.Error("GetDB error", err)
		return nil
	}

	finishDB(db, t)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tempUsersForDBTest (
				id INT NOT NULL AUTO_INCREMENT,
				name VARCHAR(45) NOT NULL,
				phone VARCHAR(45),
				time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
				PRIMARY KEY (id));`)
	if err != nil {
		t.Error("Failed to create table", err)
	}
	return db
}

func finishDB(db *db.DB, t *testing.T) {
	_, err := db.Exec(`DROP TABLE IF EXISTS tempUsersForDBTest;`)
	if err != nil {
		t.Error("Failed to create table", err)
	}
}

func BenchmarkForPool(b *testing.B) {

	b.StopTimer()
	sql := "SELECT 1002 id, '13800000001' phone"
	db, err := db.GetDB("test")
	if err != nil {
		b.Error("GetDB error", err)
		return
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		results1 := make([]map[string]interface{}, 0)
		err = db.Query(&results1, sql)
		if err != nil {
			b.Error("Query error", sql, results1, err)
		} else if results1[0]["id"].(int64) != 1002 || results1[0]["phone"].(string) != "13800000001" {
			b.Error("Result error", sql, results1, err)
		}
	}
	b.Log("OpenConnections", db.GetConnection().Stats().OpenConnections)
}

func BenchmarkForPoolParallel(b *testing.B) {

	b.StopTimer()
	sql := "SELECT 1002 id, '13800000001' phone"
	db, err := db.GetDB("test")
	if err != nil {
		b.Error("GetDB error", err)
		return
	}
	b.StartTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			results1 := make([]map[string]interface{}, 0)
			err = db.Query(&results1, sql)
			if err != nil {
				b.Error("Query error", sql, results1, err)
			} else if results1[0]["id"].(int64) != 1002 || results1[0]["phone"].(string) != "13800000001" {
				b.Error("Result error", sql, results1, err)
			}
		}
	})
	b.Log("OpenConnections", db.GetConnection().Stats().OpenConnections)
}
