package main

import (
    "fmt"
    "sync"
    "time"
)

var mu sync.Mutex
var resource int

func main() {
    go foo()
}

func foo() {
    mu.Lock()
    defer mu.Unlock()
    fmt.Println(resource)
    bar()
}

func bar() {
    mu.Lock()
    defer mu.Unlock()
    time.Sleep(1 * time.Second)
    x, y := 1, 2
    fmt.Println(x + y)
}
