package tests

import (
	"fmt"
	"github.com/ssgo/s/base"
	"github.com/ssgo/s/redis"
	"testing"
	"time"
)

type userInfo struct {
	Id    int
	Name  string
	Phone string
	Time  time.Time
}

func TestMakePasswd(t *testing.T) {

	testString := "hfjyhfjy"
	var settedKey = []byte("?GQ$0K0GgLdO=f+~L68PLm$uhKr4'=tV")
	var settedIv = []byte("VFs7@sK61cj^f?HZ")
	encrypted := base.EncryptAes(testString, settedKey, settedIv)
	fmt.Println("Redis encrypted password is:" + encrypted)
}

func TestBase(t *testing.T) {
	redis := redis.GetRedis("test")
	if redis.Error != nil {
		t.Error("GetRedis error", redis)
		return
	}

	r := redis.GET("rtestNotExists")
	if r.Error != nil && r.String() != "" || r.Int() != 0 {
		t.Error("GET NotExists", r, r.String(), r.Int())
	}

	exists := redis.EXISTS("rtestName")
	if exists {
		t.Error("EXISTS", exists)
	}

	redis.SET("rtestName", "12345")
	r = redis.GETSET("rtestName", 12345)
	if r.Error != nil && r.String() != "12345" {
		t.Error("String", r)
	}
	if r.Int() != 12345 {
		t.Error("Int", r)
	}
	if r.Float() != 12345 {
		t.Error("Float", r)
	}
	if r.Bool() != false {
		t.Error("Bool", r)
	}

	exists = redis.EXISTS("rtestName")
	if !exists {
		t.Error("EXISTS", exists)
	}

	r = redis.GET("rtestName")
	if r.Error != nil && r.String() != "12345" {
		t.Error("String", r)
	}
	if r.Int() != 12345 {
		t.Error("Int", r)
	}
	if r.Float() != 12345 {
		t.Error("Float", r)
	}

	redis.SET("rtestName", 12345.67)
	r = redis.GET("rtestName")
	if r.Error != nil && r.String() != "12345.67" {
		t.Error("String", r)
	}
	if r.Float() != 12345.67 {
		t.Error("Float", r)
	}
	if r.Uint64() != 12345 {
		t.Error("Uint64", r)
	}

	u := userInfo{
		Name: "aaa",
		Id:   123,
		Time: time.Now(),
	}
	redis.SET("rtestUser", u)
	r = redis.GET("rtestUser")
	ru := new(userInfo)
	r.To(ru)
	if r.Error != nil && ru.Name != "aaa" || ru.Id != 123 || !ru.Time.Equal(u.Time) {
		t.Error("userInfo Struct", ru)
	}

	rm := map[string]interface{}{}
	r.To(&rm)
	if rm["name"] != "aaa" || rm["id"].(float64) != 123 {
		t.Error("userInfo Map", rm)
	}

	keys := redis.KEYS("rtest*")
	if len(keys) != 2 {
		t.Error("keys", keys)
	}

	redis.MSET("rtestName", "Sam Lee", "rtestUser", map[string]interface{}{
		"name": "BBB",
	})
	results := redis.MGET("rtestName", "rtestUser")
	if len(results) != 2 || results[0].String() != "Sam Lee" {
		t.Error("MGET Results", results)
	}
	ru2 := new(userInfo)
	results[1].To(ru2)
	if ru2.Name != "BBB" {
		t.Error("MGET Struct", results)
	}

	r = redis.Do("MGET", "rtestName", "rtestUser")
	r1 := make([]string, 0)
	r.To(&r1)
	if len(r1) != 2 || r1[0] != "Sam Lee" {
		t.Error("MGET2 Strings", r1)
	}
	r2 := struct {
		RtestName string
		RtestUser userInfo
	}{}
	r.To(&r2)
	if r2.RtestName != "Sam Lee" || r2.RtestUser.Name != "BBB" {
		t.Error("MGET2 Struct and Struct", r2)
	}
	rm2 := r.ResultMap()
	if rm2["rtestName"].String() != "Sam Lee" || rm2["rtestUser"].ResultMap()["name"].String() != "BBB" {
		t.Error("MGET2 ResultMap", rm2)
	}
	ra2 := r.Results()
	if ra2[0].String() != "Sam Lee" || ra2[1].ResultMap()["name"].String() != "BBB" {
		t.Error("MGET2 ResultMap", ra2)
	}

	redis.SET("rtestIds", []interface{}{1, 2, 3})
	r = redis.GET("rtestIds")
	ria := r.Ints()
	if ria[0] != 1 || ria[1] != 2 || ria[2] != 3 {
		t.Error("userIds Ints", ria)
	}

	num := redis.DEL("rtestName", "rtestUser", "rtestIds")
	if num != 3 {
		t.Error("DEL", num)
	}
}

func TestConfig(t *testing.T) {
	redis := redis.GetRedis("localhost:6379:2:1000:500")
	fmt.Println(redis.Config)
	if redis.Error != nil {
		t.Error("GetRedis error", redis)
		return
	}

	redis.SET("rtestName", "12345")
	r := redis.GET("rtestName")
	if r.Error != nil && r.String() != "12345" {
		t.Error("String", r)
	}

	num := redis.DEL("rtestName")
	if num != 1 {
		t.Error("DEL", num)
	}
}

func TestHash(t *testing.T) {
	redis := redis.GetRedis("test")
	if redis.Error != nil {
		t.Error("GetRedis error", redis)
		return
	}

	r := redis.HGET("htest", "NotExists")
	if r.String() != "" || r.Int() != 0 {
		t.Error("GET NotExists", r, r.String(), r.Int())
	}

	exists := redis.HEXISTS("htest", "Name")
	if exists {
		t.Error("HEXISTS", exists)
	}

	redis.HSET("htest", "Name", "12345")
	r = redis.HGET("htest", "Name")
	if r.String() != "12345" {
		t.Error("String", r)
	}
	if r.Int() != 12345 {
		t.Error("Int", r)
	}
	if r.Float() != 12345 {
		t.Error("Float", r)
	}

	exists = redis.HEXISTS("htest", "Name")
	if !exists {
		t.Error("HEXISTS", exists)
	}

	redis.HSET("htest", "Name", 12345.67)
	r = redis.HGET("htest", "Name")
	if r.String() != "12345.67" {
		t.Error("String", r)
	}
	if r.Float() != 12345.67 {
		t.Error("Float", r)
	}
	if r.Uint64() != 12345 {
		t.Error("Uint64", r)
	}

	u := userInfo{
		Name: "aaa",
		Id:   123,
		Time: time.Now(),
	}
	redis.HSET("htest", "User", u)
	ru := new(userInfo)
	redis.HGET("htest", "User").To(ru)
	redis.HGET("htest", "User").To(ru)
	if ru.Name != "aaa" || ru.Id != 123 || !ru.Time.Equal(u.Time) {
		t.Error("Ints", ru)
	}

	rm := map[string]interface{}{}
	redis.HGET("htest", "User").To(&rm)
	if rm["name"] != "aaa" || rm["id"].(float64) != 123 {
		t.Error("user", rm)
	}

	redis.HMSET("htest", "Name", "Sam Lee", "User", map[string]interface{}{
		"name": "BBB",
	})
	results := redis.HMGET("htest", "Name", "User")
	if len(results) != 2 || results[0].String() != "Sam Lee" {
		t.Error("HMGET", results[0])
	}
	ru2 := new(userInfo)
	results[1].To(ru2)
	if ru2.Name != "BBB" {
		t.Error("HMGET", results[1])
	}

	r = redis.Do("HMGET", "htest", "Name", "User")
	r1 := make([]string, 0)
	r.To(&r1)
	if r.Error != nil && len(r1) != 2 || r1[0] != "Sam Lee" {
		t.Error("HMGET r1", r1)
	}

	r2 := struct {
		Name string
		User userInfo
	}{}
	r.To(&r2)
	if r2.Name != "Sam Lee" && r2.User.Name != "BBB" {
		t.Error("HMGET r2", r2)
	}

	rm3 := redis.HGETALL("htest")
	if rm3["Name"].String() != "Sam Lee" || rm3["User"].ResultMap()["name"].String() != "BBB" {
		t.Error("HGETALL ResultMap", rm3)
	}

	redis.HSET("htest", "Ids", []interface{}{1, 2, 3})
	r = redis.HGET("htest", "Ids")
	ria := r.Ints()
	if ria[0] != 1 || ria[1] != 2 || ria[2] != 3 {
		t.Error("userIds Ints", ria)
	}

	keys := redis.HKEYS("htest")
	if len(keys) != 3 {
		t.Error("HKEYS", keys)
	}

	len := redis.HLEN("htest")
	if len != 3 {
		t.Error("HLEN", keys)
	}

	num := redis.DEL("htest")
	if num != 1 {
		t.Error("DEL", num)
	}
}
