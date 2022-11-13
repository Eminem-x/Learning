# Concurrency in Go is hard

> https://medium.com/dev-genius/concurrency-in-go-is-hard-57500304650

I understand that title might be somewhat confusing as Go is generally known for having good built-in support for concurrency.

However, I don't necessarily think it's easy to write concurrent software in Go. Let me show you what I mean.

## Using global variables

The first example is something we ran into while working on a project.

Up until recently, the <a href="https://github.com/Shopify/sarama">sarama</a> library(Go library for Apache Kafka) contained the following piece of code.

````go
package sarama

import "runtime/debug"

var v string

func version() string {
    if v == "" {
        bi, ok := debug.ReadBuildInfo()
        if ok {
            v = bi.Main.Version
        } else {
            v = "dev"
        }
    }
    return v
}
````

At a glance, this looks fine. If the version isn't set globally, it's either based on the build information or assigned to a static value.

Otherwise, the version is returned as is. When  we run this code, it would appear to work as intended.

However, when the version function is called concurrently, 

the global variable v could be accessed from multiple coroutines simultaneously, resulting in a potential data race.

These issues are hard to track down as they only occur at runtime at precisely the right conditions.

#### The solution

This issue was fixed in #2171 by using `sync.once` which, according to the docs, is "an object that will perform exactly one action".

This means that we can use it to set the version once so that subsequent calls to the version function will return the result.

The fix looks like this:

````go
package sarama

import (
    "runtime/debug"
    "sync"
)

var (
    v     string
    vOnce sync.Once
)

func version() string {
    vOnce.Do(func() {
        bi, ok := debug.ReadBuildInfo()
        if ok {
           v = bi.Main.Version
        } else {
           v = "dev"
        }
    })
    return v
}
````

Although I think in this case, it could also have been fixed without using the `sync` package by using `init`.

As the variable `v` won't change after Go runs the function, it should be fine.

#### How to prevent it

You can use the <strong><a href="https://go.dev/doc/articles/race_detector">data race detector</a></strong> during tests or when using `go run` if you only want to start the application.

When it detects a potential data race, it will print a warning. See the below example:

````go
package main
import (
    "fmt"
    "runtime/debug"
)
var v string
func version() string {
    if v == "" {
        bi, ok := debug.ReadBuildInfo()
        if ok {
            v = bi.Main.Version
        } else {
            v = "dev"
        }
    }
    return v
}
func main() {
    go func() {
        version()
    }()
    fmt.Println(version())
}
````

Now we can run it with the `-race` flag to enable the race detector:

````bash
➜ go run -race .                               
==================
WARNING: DATA RACE
Write at 0x000104a16b90 by main goroutine:
  main.version()
      main.go:14 +0x78
  main.main()
      main.go:27 +0x30
Previous read at 0x000104a16b90 by goroutine 7:
  main.version()
      main.go:11 +0x2c
  main.main.func1()
      main.go:24 +0x24
Goroutine 7 (finished) created at:
  main.main()
      main.go:23 +0x2c
==================
(devel)
Found 1 data race(s)
exit status 66
````

If we analyze the output, we can see that we simultaneously read and write to the variable.

This is what we call a data race, because both goroutines are "racing" to access the same data.

-----

## Coping structs from the sync package

I'll explain this based on a sample I made. So here it goes:

````go
package main
import "sync"
type User struct {
    lock sync.RWMutex
    Name string
}
func doSomething(u User) {
    u.lock.RLock()
    defer u.lock.RUnlock()
    // do something with `u`
}
func main() {
    u := User{Name: "John"}
    doSomething(u)
}
````

The `user` struct contains two properties: a read/write lock and a string.

When the `doSomething` function is called, the variable `u` is copied on the stack, including its fields.

This is an issue because as the documentation for the sync package states:

>Package sync provides basic synchronization primitives such as mutual exclusion locks. 
>
>Other than the Once and WaitGroup types, most are intended for use by low-level library routines. 
>
>Higher-level synchronization is better done via channels and communication.
>
>**Values containing the types defined in this package should not be copied.**

When the `doSomething` function is evaluated, 

running `RLock`/`RUnlock` will not affect the original lock in the `User` struct rendering it useless.

#### The solution

Use a pointer to the lock instead. The pointer will be copied and points to the same value：`

````go
type User struct {
    lock *sync.RWMutex
    Name string
}
````

#### How to prevent it

Use the [copylock analyzer](https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/copylock) to show a warning when types from the `sync` package are copied. 

The easiest way is to run `go vet` before releasing your code.

 Running this on the original code results in the following output:

````bash
➜ go vet .
# data-synchronization
./main.go:10:20: doSomething passes lock by value: data-synchronization.User contains sync.RWMutex
./main.go:20:14: call of doSomething copies lock value: data-synchronization.User contains sync.RWMutex
````

----

## Using time.After

See the below example:

```go
var timer <-chan time.Time
if timeout > 0 {
    timer = time.After(timeout)
}

// Perform the restore.
restore := &userRestoreFuture{
    meta:   meta,
    reader: reader,
}
restore.init()
select {
case <-timer:
    return ErrEnqueueTimeout
case <-r.shutdownCh:
    return ErrRaftShutdown
case r.userRestoreCh <- restore:
    // If the restore is ingested then wait for it to complete.
    if err := restore.Error(); err != nil {
        return err
    }
}
```

This piece of code is from the `Restore` method. 

The select statement waits for one of the following cases: 

the timer (used for when a timeout is defined), a shutdown channel, 

or when the restore operation is finished. It’s pretty straightforward, so what is the issue?

The `time.After` function looks like this:

```
func After(d Duration) <-chan Time {
    return NewTimer(d).C
}
```

So it’s nothing more than a shorthand for `time.NewTimer,` but it “leaks” the timer (as there is no call to `timer.Stop`). 

The documentation says the following about it:

> After waits for the duration to elapse and then sends the current time on the returned channel. 
>
> It is equivalent to NewTimer(d).C. The underlying Timer is not recovered by the garbage collector until the timer fires. 
>
> If efficiency is a concern, use NewTimer instead and call Timer.Stop if the timer is no longer needed.

I genuinely don’t understand how a function that intentionally “leaks” the timer (resulting in a potential long-lived allocation, 

depending on the duration) ends up in the standard library…

#### The solution

Instead of using `time.After`, we can create the timer manually. This is what it looks like:

```
var timerCh <-chan time.Time
if timeout > 0 {
    timer := time.NewTimer(timeout)
    defer timer.Stop()
    timerCh = timer.C
}// Perform the restore.
restore := &userRestoreFuture{
    meta:   meta,
    reader: reader,
}
restore.init()
select {
case <-timerCh:
    return ErrEnqueueTimeout
case <-r.shutdownCh:
    return ErrRaftShutdown
case r.userRestoreCh <- restore:
    // If the restore is ingested then wait for it to complete.
    if err := restore.Error(); err != nil {
        return err
    }
}
```

When the function is finished, the timer is cleaned up even if it didn’t fire.

#### How to prevent it

I wouldn’t use `time.After` in any codebase.

 There is no real advantage other than saving one or two lines of code, 

while it can give quite some issues, mainly when it’s used in the hot paths of your code.