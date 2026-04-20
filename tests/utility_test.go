package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/ssgo/config"
	"github.com/ssgo/s"
)

func TestMakeId(tt *testing.T) {
	config.ResetConfigEnv()
	_ = os.Setenv("service_listen", ":,http")
	s.ResetAllSets()
	as := s.AsyncStart()
	ids := map[string]bool{}
	for i := 0; i < 100000; i++ {
		uid := s.MakeId(12)
		if ids[uid] {
			fmt.Println("重复", uid)
			break
		}
		ids[uid] = true
	}
	as.Stop()
}
