package tests

import (
	".."
	"github.com/ssgo/base"
	"os"
	"testing"
)

func List(in struct{}) s.Map {
	return s.Map{
		"code": 1,
		"list": []ItemA{makeItemA(), makeItemA(), makeItemA(), makeItemA(), makeItemA(), makeItemA(), makeItemA(), makeItemA(), makeItemA(), makeItemA(), makeItemA(), makeItemA(), makeItemA(), makeItemA(), makeItemA()},
	}
}

type ItemA struct {
	Index int
	List  []ItemB
}
type ItemB struct {
	Password int
}

var indexA, indexB int

func makeItemA() ItemA {
	indexA++
	return ItemA{
		Index: indexA,
		List:  []ItemB{makeItemB(), makeItemB(), makeItemB(), makeItemB(), makeItemB(), makeItemB(), makeItemB(), makeItemB(), makeItemB(), makeItemB(), makeItemB(), makeItemB(), makeItemB(), makeItemB(), makeItemB()},
	}
}
func makeItemB() ItemB {
	return ItemB{
		Password: base.Rander.Int(),
	}
}

func TestList(tt *testing.T) {
	t := s.T(tt)

	os.Setenv("service_logOutputFields", "code,list")
	s.ResetAllSets()
	s.Register(0, "/list", List)
	as := s.AsyncStart()

	r := as.Post("/list?a=1", s.Map{"b": s.Arr{1, 2, 3, 4, 5}})
	t.Test(r.Error == nil, "list", r.Error, r.String())

	as.Stop()
}
