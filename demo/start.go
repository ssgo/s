package main

import "github.com/ssgo/s"

func main() {
	s.Register(0, "/", func() string {
		return "Hello\n"
	})
	s.Start1()
}