package main

import (
    "fmt"
    "os"
)

// test break from select
func main() {
    abort := make(chan byte)

    go func() {
        c := make([]byte, 1)
        for {
            os.Stdin.Read(c)
            if (c[0] != '\n') {
                abort <- c[0]
            }
        }
    }()

    for {
        select {
        case x := <-abort:
            if x == '0' {
                fmt.Println("Break!")
                break
            } else {
                fmt.Println("Loop...")
            }
        }
    }
}
