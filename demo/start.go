package main

import (
	".."
	"./userServices"
)

func main(){
	s.Register(0, "/", userServices.Index)
	s.Start()
}
