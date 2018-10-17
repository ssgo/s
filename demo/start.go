package main

import (
	"os"

	"github.com/ssgo/s"
)

func main() {
	os.Setenv("service_listen", ":8801")
	s.Register(0, "/", func() string {
		return "Hello\n"
	})
	s.Start()
}
