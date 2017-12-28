package main

import (
	".."
	"./userServices"
)

func main(){
	s.Register("/", userServices.Index)
	s.Start()
}
