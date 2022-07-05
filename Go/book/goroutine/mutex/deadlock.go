package main

import (
    "fmt"
    "sync"
)

var mu sync.Mutex

func main() {
    var resource int
    mu.Lock()
    fmt.Println(resource)
    mu.Lock()
    fmt.Println(resource)
    mu.Unlock()
    mu.Unlock()
}
