package base

import (
	"github.com/ssgo/base"
	"os"
	"testing"
)

func TestForStructWithBadEnv(t *testing.T) {
	os.Setenv("TEST_NAME", "ttt")
	base.ResetConfigEnv()
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
	os.Unsetenv("TEST_NAME")
}

func TestForStructWithEnv(t *testing.T) {
	os.Setenv("TEST_NAME", "\"ttt\"")
	os.Setenv("TEST_SETS_1", "222")
	base.ResetConfigEnv()
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
	os.Unsetenv("TEST_NAME")
	os.Unsetenv("TEST_SETS_1")
}

func TestForStructWithEnvForSlice(t *testing.T) {
	os.Setenv("TEST_NAME", "\"ttt\"")
	os.Setenv("TEST_SETS", "[11,22,33]")
	base.ResetConfigEnv()
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
	os.Unsetenv("TEST_NAME")
	os.Unsetenv("TEST_SETS")
}
