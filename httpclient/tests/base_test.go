package tests

import (
	"testing"

	"github.com/ssgo/s/httpclient"
)

func Hello() string {
	return "Hello"
}

func TestHttp(tt *testing.T) {
	c := httpclient.GetClient(0)
	r := c.Get("http://61.135.169.121")
	if r.Error != nil {
		tt.Error("baidu error	", r.Error)
	}
}
