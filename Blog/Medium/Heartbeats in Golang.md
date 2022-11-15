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