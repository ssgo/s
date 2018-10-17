package main

import "github.com/ssgo/s"

func getFullName(in struct{ Name string }) (out struct{ FullName string }) {
	out.FullName = in.Name + " Lee"
	return
}

func main() {
	s.Register(1, "/{name}/fullName", getFullName)
	s.Start()
}
