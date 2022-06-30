package main

import "fmt"

func main() {
    abort := make(chan struct{})
    select {
    case <-abort:
        fmt.Println(123)
        return
    default:
        // no action
    }
    fmt.Println(456)
}
