package main

import (
	"github.com/ssgo/s"
	"github.com/ssgo/s/discover"
)

func getInfo(in struct{ Name string }, c *discover.Caller) (out struct{ FullName string }) {
	c.Get("s1", "/"+in.Name+"/fullName").To(&out)
	return
}

func main() {
	s.Register(0, "/{name}", getInfo)
	s.Start1()
}
