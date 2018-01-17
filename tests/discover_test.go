package tests

import (
	"testing"
	".."
	"os"
)

func S1() string{
	return "s1"
}

func C1(c *s.Caller) string{
	return c.Call("ta", "/s1", nil).String()
}

func TestBase(tt *testing.T) {
	t := s.T(tt)

	s.ResetAllSets()
	s.Register(1, "/c1", C1)
	s.Register(2, "/s1", S1)
	os.Setenv("SERVICE_APP", "ta")
	os.Setenv("SERVICE_WEIGHT", "100")
	os.Setenv("SERVICE_ACCESSTOKENS", `{"aabbcc": 1, "aabbcc222": 2}`)
	os.Setenv("SERVICE_CALLS", `{"ta": {"accessToken": "aabbcc222", "timeout": 200}}`)
	addr := s.AsyncStart()

	c := s.GetClient()
	for i := 0; i < 1; i++ {
		r := c.Do("http://"+addr+"/c1", nil, "Access-Token", "aabbcc")
		t.Test(r.Error == nil && r.String() == "s1", "DC", r.Error, r.String())
	}

	s.Stop()
	s.WaitForAsync()
}
