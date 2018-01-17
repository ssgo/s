package main

import "github.com/ssgo/s"

func getInfo(in struct{ Name string }, c *s.Caller) (out struct{ FullName string }) {
	c.Call("s1", "/"+in.Name+"/fullName", nil).To(&out)
	return
}

func main() {
	s.Register(0, "/{name}", getInfo)
	s.Start1()
}
