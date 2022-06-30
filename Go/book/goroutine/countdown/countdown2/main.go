package main

import (
    "fmt"
    "os"
    "time"
)

func main() {
    abort := make(chan struct{})
    go func() {
        os.Stdin.Read(make([]byte, 1))
        abort <- struct{}{}
    }()

    fmt.Println("Connecting countdown. Press return to abort.")

    go func() {
        tick := time.Tick(1 * time.Second)
        for i := 0; i < 10; i++ {
            fmt.Println(i + 1)
            <-tick
        }
    }()

    select {
    case <- time.After(10 * time.Second):
        fmt.Println("Launch!")
    case <- abort:
        fmt.Println("Launch aborted!")
    }
}
