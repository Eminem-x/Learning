# Several Golang Skills You Must Know

> https://medium.com/@Carlos_go/several-golang-skills-you-must-know-d60a0344a28f

## Avoid using reflection

Avoid using reflection to make programs more efficient. Go is a very nice programming language.

It's a language that lets you really focus on your business and don't worry too much about the language itself,

so you can write applications as quickly as possible. For example,

it has a relatively complete ecosystem that provides you with everything you need to get started.

However, it's not a panacea, there are some flaws to be aware of. Maybe just tricks, not really a problem.

If you notice them and use them in programming, they will benefit you a lot,

which is the difference between a novice and veteran.

### Don't use Logrus

This is actually related to generics. Because Go is strongly, statically typed language.

What if we also need to use generic types? For example, like Logger, or ORM, only with common types,

common code can be written, otherwise, each type of function would have to be written once.

Ultimately, we can only use reflection. And Logrus makes heavy use of  reflection, which result in lots of allocation counts.

While usually not a  huge issue(depending on the code), performance matters,

especially in large-scale, high-concurrency projects. While this sounds like a very small optimization,

it's important to avoid reflections. If you see some code that uses structs regardless of type,

it uses reflection and has a performance impact. Logrus doesn't care about types but apparently Go needs to know.

Logrus use reflection to detect the type, which is overhead.

<strong>So I would prefer zerolog, of course, zap is also good. </strong>

Both clain zero allocations, which is also we want, since they have a minimal performance impact.

### Don't use encoding/json

When we need a function, many people suggest using the standard library.

The exception in the encoding/json module in the standard library.

In fact, encoding/json uses reflection, 

which will lead to low performance and cause losses when writing APIs or micro services that return json responses.

For example you can use Easyjson, it's simple and efficient, 

it use a code generator to create the code needed from structure to json to minimize allocation.

This is manual build step and it's annoying. Interestingly json-iterator also uses reflection, but it significantly faster.

### Try not to use closures in goroutines

```go
for i := 0; i < 10; i++ {
	go func() {
		fmt.Println(i)
	}()
}
```

Most people would probably except this to print the numbers 0 through 9, just like when delegating the task to a goroutine.

But the actual result: depending on the system, you will get one or two numbers and many 10.

Closures have access to the parent scope, so variables can be used directly.

Although new linters may warn you about "variable closure capture". It doesn't requiere you to redeclare the variable.

Go’s performance reputation is largely due to runtime optimization of execution, 

which tries to “guess” what you want to do and optimize certain execution paths. 

During this time, it “captures” variables and passes them where they are needed in the theoretically most efficient way 

(e.g. after some non-concurrent operation is done to free allocations on some CPU). 

The consequence of this is that the loop may start goroutines that may receive the value of i from the parent scope much later. 

There’s no guarantee which one you’ll see when executing this code multiple times, it could be the number 10, or something else.

If you do use a closure for some reason, be sure to pass the variable i, and treat the closure like every function.

```go
for i := 0; i < 10; i++ {
	go func() {
    i := i
		fmt.Println(i)
	}()
}
```

