package main

import (
    "fmt"
)

func main() {
    // c := make(chan int)
    c := make(chan int, 1)
    c <- 0
    foo()
    bar()
}

func foo() {
    fmt.Println("foo")
}

func bar() {
    fmt.Println("bar")
}
