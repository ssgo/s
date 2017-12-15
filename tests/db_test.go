package tests

import _ "github.com/go-sql-driver/mysql"
import(
	db ".."
	"testing"
)

type userInfo struct {
	Phone  string
	Name   string
	UserId int
}

func TestBaseSelect(t *testing.T){

	sql := "SELECT 1002 userId, '13800000001' phone"
	db, err := db.GetDB("test")
	if err != nil {
		t.Error("GetDB error", err)
		return
	}
	defer db.Close()

	results1 := make([]map[string]interface{}, 0)
	err = db.Query( &results1, sql)
	if err != nil{
		t.Error("Query error", sql, results1, err)
	}else if results1[0]["userId"].(int64) != 1002 || results1[0]["phone"].(string) != "13800000001" {
		t.Error("Result error", sql, results1, err)
	}

	results2 := make([]map[string]string, 0)
	err = db.Query( &results2, sql)
	if err != nil{
		t.Error("Query error", sql, results2, err)
	}else if results2[0]["userId"] != "1002" || results2[0]["phone"] != "13800000001" {
		t.Error("Result error", sql, results2, err)
	}

	results3 := make([]map[string]int, 0)
	err = db.Query( &results3, sql)
	if err != nil{
		t.Error("Query error", sql, results3, err)
	}else if results3[0]["userId"] != 1002 || results3[0]["phone"] != 13800000001 {
		t.Error("Result error", sql, results3, err)
	}

	results4 := make([]userInfo, 0)
	err = db.Query( &results4, sql)
	if err != nil{
		t.Error("Query error", sql, results4, err)
	}else if results4[0].UserId != 1002 || results4[0].Phone != "13800000001" {
		t.Error("Result error", sql, results4, err)
	}

	results5 := make([][]string,0)
	err = db.Query( &results5, sql)
	if err != nil{
		t.Error("Query error", sql, results5, err)
	}else if results5[0][0] != "1002" || results5[0][1] != "13800000001" {
		t.Error("Result error", sql, results5, err)
	}

	results6 := make([]string,0)
	err = db.Query( &results6, sql)
	if err != nil{
		t.Error("Query error", sql, results6, err)
	}else if results6[0] != "1002" {
		t.Error("Result error", sql, results6, err)
	}

	results7 := map[string]interface{}{}
	err = db.Query( &results7, sql)
	if err != nil{
		t.Error("Query error", sql, results7, err)
	}else if results7["userId"].(int64) != 1002 || results7["phone"] != "13800000001" {
		t.Error("Result error", sql, results7, err)
	}

	results8 := userInfo{}
	err = db.Query( &results8, sql)
	if err != nil{
		t.Error("Query error", sql, results8, err)
	}else if results8.UserId != 1002 || results8.Phone != "13800000001" {
		t.Error("Result error", sql, results8, err)
	}

	var results9 int
	err = db.Query( &results9, sql)
	if err != nil{
		t.Error("Query error", sql, results9, err)
	}else if results9 != 1002 {
		t.Error("Result error", sql, results9, err)
	}
}
