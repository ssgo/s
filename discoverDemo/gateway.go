package main

import (
	"github.com/ssgo/discover"
	"github.com/ssgo/s"
)

func getInfo(in struct{ Name string }, c *discover.Caller) (out struct{ FullName string }) {
	_ = c.Get("s1", "/"+in.Name+"/fullName").To(&out)
	return
}

func main() {
	s.Register(0, "/{name}", getInfo)
	s.Start()
}
