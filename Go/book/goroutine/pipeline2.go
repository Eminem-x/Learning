package main

import "fmt"
import "time"

func main() {
    naturals := make(chan int)
    squares := make(chan int)

    // counter
    go func() {
        for x := 0; x < 10; x++ {
            naturals <- x
        }
        close(naturals)
    }()

    // squarer
    go func() {
        for x := range naturals {
            squares <- x * x
        }
        close(squares)
    }()

    // printer
    for x := range squares {
        time.Sleep(1 * time.Second)
        fmt.Println(x)
    }
}
