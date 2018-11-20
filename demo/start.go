package main

import (
	"github.com/ssgo/s"
)

func main() {
	/**s.Register(0, "/", func() string {
		return "Hello\n"
	})**/
	s.Restful(0, "GET", "/hello", func() string {
		return "Hello ssgo\n"
	})
	s.Start1()
}
