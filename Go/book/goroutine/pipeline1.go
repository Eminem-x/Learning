package main

import "fmt"
import "time"

func main() {
    naturals := make(chan int)
    squares := make(chan int)

    // counter
    go func() {
        for x := 0; ; x++ {
            naturals <- x
        }
    }()

    // squarer
    go func() {
        for {
            x := <-naturals
            squares <- x * x
        }
    }()

    // printer
    for {
        time.Sleep(1 * time.Second)
        fmt.Println(<-squares)
    }
}
