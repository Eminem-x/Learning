# Heartbeats in Golang

> https://medium.com/geekculture/heartbeats-in-golang-1a12c4c366f
>
> code repository：https://github.com/Eminem-x/Learning/tree/main/Go/book/learn/heartbeats

## What are Heartbeats?

Heartbeats are a way for concurrent processes to signal life to outside parties.

They get their name from human anatomy wherein a heartbeat signifies file to an observer.

Heartbeats have been around since before Go, and remain useful within it.

There are a few diffierent reasons heartbeats are interestring for concurrent code.

They allow us insights into out system, and they can make testing the system deterministic when it might otherwise not be.

There are two different types of heartbeats:

* <strong>Heartbeats that occur at a time interval</strong>
* <strong>Heartbeats occur at the beginning of a uint of work</strong>

----

### Heartbeats that occur at a time interval

Heartbeats that occur on a time interval are useful for concurrent code 

that might be waiting for something else to happen for it to process a unit of work.

Because you don't know when that work might come in,

your goroutine might be sitting around for a while waitting for something to happen.

A heartbeat is a way to single to its listeners that everything is well and that silence is expected.

The following code demonstrates <strong>a goroutine that exposes a heartbeat:</strong>

```go
doWork := func(
    done <-chan interface{},
    pulseInterval time.Duration,
) (<-chan interface{}, <-chan time.Time) {
    heartbeat := make(chan interface{})
    results := make(chan time.Time)
    go func() {
        defer close(heartbeat)
        defer close(results)

        pulse := time.Tick(pulseInterval)
        workGen := time.Tick(2*pulseInterval)

        sendPulse := func() {
            select {
            case heartbeat <-struct{}{}:
            default:
            }
        }
        sendResult := func(r time.Time) {
            for {
                select {
                case <-done:
                    return
                case <-pulse:
                    sendPulse()
                case results <- r:
                    return
                }
            }
        }

     		// for i := 0; i < 10; i++ {} simulta an incorrectly goroutine
        for {
            select {
            case <-done:
                return
            case <-pulse:
                sendPulse()
            case r := <-workGen:
                sendResult(r)
            }
        }
    }()
    return heartbeat, results
}
```

Notice that because we might be sending out multiple pulses while we wait for input,

or multiple pluses while waiting to send results, all the `select` statements need to be within `for` loops.

Looking good so far, how do we utilize this function and consume the events it emits? Let's take a look:

```go
done := make(chan interface{})
time.AfterFunc(10*time.Second, func() { close(done) }) 

const timeout = 2*time.Second
heartbeat, results := doWork(done, timeout/2)
for {
    select {
    case _, ok := <-heartbeat:
        if ok == false {
            return
        }
        fmt.Println("pulse")
    case r, ok := <-results:
        if ok == false {
            return
        }
        fmt.Printf("results %v\n", r.Second())
    case <-time.After(timeout):
      	fmt.Println("worker goroutine is not healthy!")
        return
    }
}
```

 running this code produces:

```go

pulse
pulse
results 52
pulse
pulse
results 54
pulse
pulse
results 56
pulse
pulse
results 58
pulse
```

You can see that we receive about two pulses per result as we intended.

Now ina properly functioning system, heartbeats aren't that interesting.

We might use them to gather statistics regarding idle time, 

but the utility for interval-based heartbeats really shines when your goroutine isn't behaving as expected.

By using a heartbeat, successfully avoid a deadlock, and we remain deterministic by not having to rely on a longer timeout.

Also, note that heartbeats help with the opposite case: they let us know that long-running goroutines remain up, 

but are just taking a while to produce a value to send on a longer timeout.

-----

### Heartbeats occur at the beginning of a uint of work

Now let's shift over to looking at heartbeats that occur at the beginning of a unit of work.

These are extremely useful for tests. Here's an example that sends a pulse before every unit of work:

```go
doWork := func(done <-chan interface{}) (<-chan interface{}, <-chan int) {
    heartbeatStream := make(chan interface{}, 1)
    workStream := make(chan int)
    go func () {
        defer close(heartbeatStream)
        defer close(workStream)

        // time.Sleep(2*time.Second)
      
        for i := 0; i < 10; i++ {
            select { 
            case heartbeatStream <- struct{}{}:
            default:
            }

            select {
            case <-done:
                return
            case workStream <- rand.Intn(10):
            }
        }
    }()

    return heartbeatStream, workStream
}

done := make(chan interface{})
defer close(done)

heartbeat, results := doWork(done)
for {
    select {
    case _, ok := <-heartbeat:
        if ok {
            fmt.Println("pulse")
        } else {
            return
        }
    case r, ok := <-results:
        if ok {
            fmt.Printf("results %v\n", r)
        } else {
            return
        }
    }
}
```

and you can see the result is:

```go
pulse
results 1
pulse
results 7
pulse
results 7
pulse
results 9
```

You can see in this example that we receive one pulse for every result, as intended.

Where this technique realyy shines is in writing tests.

Interval-based heartbeats can be used in the same fashion, but if you only care that the goroutine has started doing its work,

this style of heartbeat is simple. Consider the following bad test code:

````go
func TestDoWork_GeneratesAllNumbers(t *testing.T) {
    done := make(chan interface{})
    defer close(done)

    intSlice := []int{0, 1, 2, 3, 5}
    _, results := DoWork(done, intSlice...)

    for i, expected := range intSlice {
        select {
        case r := <-results:
            if r != expected {
                t.Errorf(
                  "index %v: expected %v, but received %v,",
                  i,
                  expected,
                  r,
                )
            }
        case <-time.After(1 * time.Second): 
            t.Fatal("test timed out")
        }
    }
}

func DoWork(
    done <-chan interface{},
    nums ...int,
) (<-chan interface{}, <-chan int) {
    heartbeat := make(chan interface{}, 1)
    intStream := make(chan int)
    go func() {
        defer close(heartbeat)
        defer close(intStream)

        time.Sleep(2*time.Second)

        for _, n := range nums {
            select {
            case heartbeat <- struct{}{}:
            default:
            }

            select {
            case <-done:
                return
            case intStream <- n:
            }
        }
    }()

    return heartbeat, intStream
}
````

and running this test produces:

```
go test ./bad_concurrent_test.go
  --- FAIL: TestDoWork_GeneratesAllNumbers (1.00s)
      bad_concurrent_test.go:46: test timed out
  FAIL
  FAIL    command-line-arguments  1.002s
```

This test is bad because it's non-deterministic. In example function, I've ensured this test will fail,

but if I were to remove the `time.Sleep`，the situation actually gets worse: this test will pass at times, and fail at others.

This is an awful, awful position to be in. 

The team no longer knows whether it can trust a test failure and begin ignoring failures the whole endeavor begins to unravel.

Fortunately, with a heartbeat, this is easily solved. Here is a test that is deterministic:

```go
func TestDoWork_GeneratesAllNumbers(t *testing.T) {
    done := make(chan interface{})
    defer close(done)

    intSlice := []int{0, 1, 2, 3, 5}
    heartbeat, results := DoWork(done, intSlice...)

    <-heartbeat

    i := 0
    for r := range results {
        if expected := intSlice[i]; r != expected {
            t.Errorf("index %v: expected %v, but received %v,", i, expected, r)
        }
        i++
    }
}
```

and the result is:

```
ok  command-line-arguments   2.002s
```

Because of the heartbeat, we can safely write our test without timeouts.

The only risk we run is of one of our iterations taking an inordinate amount of time.

If that’s important to us, we can utilize the safer interval-based heartbeats and achieve perfect safety.

Here is an example of a test utilizing interval-based heartbeats:

```go
func DoWork(
    done <-chan interface{},
    pulseInterval time.Duration,
    nums ...int,
) (<-chan interface{}, <-chan int) {
    heartbeat := make(chan interface{}, 1)
    intStream := make(chan int)
    go func() {
        defer close(heartbeat)
        defer close(intStream)

        time.Sleep(2*time.Second)

        pulse := time.Tick(pulseInterval)
        numLoop:
        for _, n := range nums {
            for {
                select {
                case <-done:
                    return
                case <-pulse:
                    select {
                    case heartbeat <- struct{}{}:
                    default:
                    }
                case intStream <- n:
                    continue numLoop
                }
            }
        }
    }()

    return heartbeat, intStream
}

func TestDoWork_GeneratesAllNumbers(t *testing.T) {
    done := make(chan interface{})
    defer close(done)

    intSlice := []int{0, 1, 2, 3, 5}
    const timeout = 2*time.Second
    heartbeat, results := DoWork(done, timeout/2, intSlice...)

    <-heartbeat

    i := 0
    for {
        select {
        case r, ok := <-results:
            if ok == false {
                return
            } else if expected := intSlice[i]; r != expected {
                t.Errorf(
                    "index %v: expected %v, but received %v,",
                    i,
                    expected,
                    r,
                )
            }
            i++
        case <-heartbeat:
        case <-time.After(timeout):
            t.Fatal("test timed out")
        }
    }
}
```

You’ve probably noticed that this version of the test is much less clear.

The logic of what we’re testing is a bit muddled. For this reason, if you’re reasonably sure the goroutine’s loop won’t stop 

executing once it’s started I recommend only blocking on the first heartbeat and then falling into a simple `range` statement. 

You can write separate tests that specifically test for failing to close channels, loop iterations taking too long, 

and any other timing-related issues.