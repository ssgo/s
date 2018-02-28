package base

import (
	".."
	"os"
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
		t.Error("list in test.json failed", testConf.List["aaa"])
	}
}

func TestForMapWithEnv(t *testing.T) {
	os.Setenv("TEST_NAME", "\"ttt\"")
	testConf := new(map[string]interface{})
	err := base.LoadConfig("test", &testConf)
	if err != nil {
		t.Error("read test.json failed", err)
	}
	if (*testConf)["name"] != "test-config" {
		t.Error("name in test.json failed", (*testConf)["name"])
	}
}

func TestForStructWithBadEnv(t *testing.T) {
	os.Setenv("TEST_NAME", "ttt")
	testConf := testConfType{}
	err := base.LoadConfig("test", &testConf)
	if err != nil {
		t.Error("read test.json failed", err)
	}
	if testConf.Name != "ttt" {
		t.Error("name in test.json failed", testConf.Name)
	}
	if len(testConf.Sets) != 3 || testConf.Sets[1] != 2 {
		t.Error("sets in test.json failed", testConf.Sets)
	}
	if testConf.List["aaa"].Name != "222" {
		t.Error("list in test.json failed", testConf.Sets)
	}
}

func TestForStructWithEnv(t *testing.T) {
	os.Setenv("TEST_NAME", "\"ttt\"")
	os.Setenv("TEST_SETS_1", "222")
	testConf := testConfType{}
	err := base.LoadConfig("test", &testConf)
	if err != nil {
		t.Error("read test.json failed", err)
	}
	if testConf.Name != "ttt" {
		t.Error("name in test.json failed", testConf.Name)
	}
	if len(testConf.Sets) != 3 || testConf.Sets[1] != 222 {
		t.Error("sets in test.json failed", testConf.Sets)
	}
	os.Unsetenv("TEST_SETS_1")
}

func TestForStructWithEnvForSlice(t *testing.T) {
	os.Setenv("TEST_NAME", "\"ttt\"")
	os.Setenv("TEST_SETS", "[11,22,33]")
	testConf := testConfType{}
	err := base.LoadConfig("test", &testConf)
	if err != nil {
		t.Error("read test.json failed", err)
	}
	if testConf.Name != "ttt" {
		t.Error("name in test.json failed", testConf.Name)
	}
	if len(testConf.Sets) != 3 || testConf.Sets[1] != 22 {
		t.Error("sets in test.json failed", testConf.Sets)
	}
}
