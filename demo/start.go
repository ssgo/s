package main

import (
	".."
	"./userServices"
)

func main(){
	service.Register("/", userServices.Index)
	service.Start()
}
