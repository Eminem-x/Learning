package main

import (
    "fmt"
    "sync"
)

var (
    mu      sync.Mutex
    balance int
)

func Deposit(amount int) {
    mu.Lock()
    balance = balance + amount
    mu.Unlock()
}

func Balance() int {
    mu.Lock()
    defer mu.Unlock()
    return balance
}

func WithDraw(amount int) bool {
    mu.Lock()
    defer mu.Unlock()
    Deposit(-amount)
    if Balance() < 0 {
        Deposit(amount)
        return false
    }
    return true
}

func main() {
    go func() {
        Deposit(200)
        fmt.Println("Balance = ", Balance())
    }()

    go WithDraw(100)
}
