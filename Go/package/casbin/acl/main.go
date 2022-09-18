package main

import (
	"fmt"
	"log"

	"github.com/casbin/casbin/v2"
)

func main() {
	e, err := casbin.NewEnforcer("./model.conf", "./policy.csv")
	if err != nil {
		log.Fatalf("NewEnforcer failed:%#v", err)
	}
	check(e, "yh", "data1", "read")
	check(e, "ycx", "data2", "write")
	check(e, "yh", "data1", "write")
	check(e, "ycx", "data2", "read")

	check(e, "root", "data", "anything")
}

func check(e *casbin.Enforcer, sub, obj, act string) {
	ok, _ := e.Enforce(sub, obj, act)
	if ok {
		fmt.Printf("%s Can %s %s\n", sub, act, obj)
	} else {
		fmt.Printf("%s Cannot %s %s\n", sub, act, obj)
	}
}
