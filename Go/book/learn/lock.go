package main

import (
	"fmt"
	"sync"
	"time"
)

var (
	x    int64
	lock sync.Mutex
)

func main() {
	add()
}

func add() {
	x = 0
	for i := 0; i < 5; i++ {
		go addWithoutLock()
	}
	time.Sleep(5 * time.Second)
	fmt.Println("WithoutLock:", x)

	x = 0
	for i := 0; i < 5; i++ {
		go addWithLock()
	}
	time.Sleep(5 * time.Second)
	fmt.Println("WithLock:", x)
}

func addWithLock() {
	for i := 0; i < 2000; i++ {
		lock.Lock()
		x += 1
		lock.Unlock()
	}
}

// without lock make result unpredictable
func addWithoutLock() {
	for i := 0; i < 2000; i++ {
		x += 1
	}
}
