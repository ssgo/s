package base

import (
	".."
	"testing"
)

func TestForMap(t *testing.T) {
	testConf := map[string]interface{}{}
	err := base.LoadConfig("test", &testConf)
	if err != nil {
		t.Error("read test.json failed", err)
	}
	if testConf["name"] != "test-config" {
		t.Error("name in test.json failed", testConf["name"])
	}
}

type testConfType struct {
	Name string
	Sets []int
	List map[string]*struct {
		Name string
	}
	List2 map[string]string
}

func TestForStruct(t *testing.T) {
	testConf := testConfType{}
	err := base.LoadConfig("test", &testConf)
	if err != nil {
		t.Error("read test.json failed", err)
	}
	if testConf.Name != "test-config" {
		t.Error("name in test.json failed", testConf.Name)
	}
	if len(testConf.Sets) != 3 || testConf.Sets[1] != 2 {
		t.Error("sets in test.json failed", testConf.Sets)
	}
	if testConf.List["aaa"].Name != "222" {
		t.Error("map in test.json failed", testConf.List["aaa"])
	}
	if testConf.List["bbb"] == nil || testConf.List["bbb"].Name != "xxx" {
		t.Error("map in env.json failed", testConf.List)
	}
}
