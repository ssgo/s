package main

import (
	"github.com/ssgo/discover"
	"github.com/ssgo/s"
)

//func getInfo(in struct{ Name string }, c *discover.Caller) (out struct{ FullName string }) {
//	c.Get("c1", "/"+in.Name+"/fullName").To(&out)
//	return
//}

func getFullNameController(in struct{ Name string }, c *discover.Caller) (out struct{ FullName string }) {
	r := struct{ FullName string }{}
	c.Get("s1", "/"+in.Name+"/fullName").To(&r)
	out.FullName = r.FullName
	return
}

func main() {
	s.Register(1, "/{name}/fullName", getFullNameController)
	s.Start()
}
