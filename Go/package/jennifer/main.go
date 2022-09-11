package main

import (
	"fmt"

	. "github.com/dave/jennifer/jen"
)

// Official: https://github.com/dave/jennifer
// Blog: https://darjun.github.io/2020/03/14/godailylib/jennifer/
func main() {
	basic()
	value()
}

func basic() {
	f := NewFile("main")
	f.Func().Id("main").Params().Block(
		Qual("fmt", "Println").Call(Lit("Hello World")),
	)
	fmt.Printf("%#v\n", f)
}

func value() {
	f := NewFile("main")
	f.Func().Id("main").Params().Block(
		Id("greeting").Op(":=").Lit("Hello World"),
		Qual("fmt", "Println").Call(Id("greeting")),
	)
	fmt.Printf("%#v\n", f)
}
