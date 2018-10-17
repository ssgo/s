package main

import (
	"github.com/ssgo/s"
	"os"
)

func main() {
	os.Setenv("service_listen", ":8801")
	s.Register(0, "/", func() string {
		return "Hello\n"
	})
	s.Start()
}
