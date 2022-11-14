package main

import (
	"fmt"
	"time"
)

const timeout = 2 * time.Second

var (
	heartbeat = make(chan interface{})
	results   = make(chan time.Time)
	pulse     = time.Tick(timeout / 2)
	workGen   = time.Tick(timeout)
)

func main() {
	done := make(chan interface{})
	defer close(heartbeat)
	defer close(results)

	time.AfterFunc(10*time.Minute, func() { close(done) })
	go doWork(done)

	printHeart()
}

func doWork(done <-chan interface{}) {
	// If you want to simulate an incorrect goroutine, you can reduce loop times.
	for {
		// How to ensure execute pulse before wokenGen
		select {
		case <-done:
			return
		case <-pulse:
			sendPulse()
		case r := <-workGen:
			sendResult(done, r)
		}
	}
}

func sendPulse() {
	heartbeat <- struct{}{}
}

func sendResult(done <-chan interface{}, r time.Time) {
	select {
	case <-done:
		return
	case results <- r:
		return
	}
}

func printHeart() {
	var x, y int
	for {
		select {
		case _, ok := <-heartbeat:
			if !ok {
				return
			}
			x++
			fmt.Printf("pulse\n")
		case _, ok := <-results:
			if !ok {
				return
			}
			y++
			if x/y != 2 {
				fmt.Println("false")
				return
			} else {
				fmt.Printf("results\n")
			}
		case <-time.After(timeout):
			fmt.Println("worker goroutine is not healthy!")
			return
		}
	}
}
