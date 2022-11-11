# Golang — Handling errors the rigth way

> https://medium.com/@self.bordin/golang-treating-errors-the-right-way-f503f00fd55
>
> https://chai2010.cn/advanced-go-programming-book/ch1-basic/ch1-07-error-and-panic.html

## Introduction

As you may already know, Golang has a different way of treating errors, there is no try/catch,

you must explicitly check for errors right after calling a function. Like the following example:

```go
func main() {
		_, err := fmt.Println("Hello World")
		if err != nil {
				panic(err)
		}
}
```

It prints a message on the standard output and returns the number of bytes written as the 1st argument,

along with any write errors encountered, as the 2nd argument.

As in this example we don't care about the amount of bytes written, we put an underline on the first argument,

and as we want to handle the error, we create a variable named <strong>err</strong> on the second argument.

Then we just check if the error is not nil; it there is an error. If so, we panic the whole application with the error message.

## Understanding Panic

When you call panic, it immediately calls all the defers defined in the function, 

it means that, if we had a defer on the main function, it would be executed before the panic.

Defers run in the reverse order that you define, it means that the last written is the first executed.

## Panicing a complex application

Now, let's imagine that you have a robust system with a lot of nested functions.

What would happen if it would panic in the deepest function? Take a look at the example below:

```go
func firstLevel() {
		defer fmt.Println("Exiting first level")
		secondLevel()
}

func secondLevel() {
		defer fmt.Println("Exiting second level")
		thirdLevel()
}

func thirdLevel() {
		defer fmt.Println("Exiting third level")
		fourthLevel()
}

func fourthLevel() {
		defer fmt.Println("Exiting fourth level")
		panic("Panicing on fourth level")
}

func main() {
		firstLevel()
}
```

When the application hits the panic, it will firstly call the defers defined and then panic the current context.

As the context of the panic is the fourthLevel function, and not the main, the application won't exit yet.

It will kill the function and return back to the thirdLevel. Then, thirdLevel will also suffer the same panic behaviours;

call the defers and get killed, and so on until the panic hits the main function,

and does not find any other defer to execute or function to return on.

## Third party library

When we have an error on some major part of the application, 

like on the startup when it connects to important service in order to work, 

it is ok to panic, as the application won't work if we don't do that.

But what do we do if there is a panic in some third party library that we cannot have control over the code,

and we don't wanna let it crash the application? As we saw on the example before,

panic works like a waterfall, crashing each caller function, so if a panic happens inside a third party library,

it would crash the whole application, as the library's function is being called.

## Understanding recover

Recover listens to panics and regains the control of the execution, keeping the application up and letting the flow continue.

Recovers must be defined in defers at the very beginning of the function.

So when the function ends, due to a normal execution or due to a panic, 

the defer will be executed and recover will check if some panic happened, 

if so, it will let you handle the error and won't let the application crash. See the example below:

````go
func main() {
    defer func() {
        if r := recover(); r != nil { ... }
        // 虽然总是返回 nil, 但是可以恢复异常状态
    }()

    // 警告: 用 nil 为参数抛出异常
    panic(nil)
}
````

It can execute this defer and the panic gets recovered, stopping if from crashing all the functions.

Then the flow backs to the main function like nothing happened, and main can continue with its flow.

## Recover and Goroutine

Recovers only work if the panic happens on the same routine as they are defined.

It means that if a function A calls a goroutine B, and the panic happens inside B, recover won't catch it.

See the example below:

```go
func main() {
		defer func() {
				if r := recover(); r != nil {
						...
				}
		}()
		go Panic()
		time.Sleep(3 * time.Second) // not recommend
}

func Panic() {
		panic(1)
}
```

As you can see, we triggered a goroutine to execute Panic. but what about recover?

Wasn't it executed when panic called the defer? Yes, recover got executed, but it could not find any panic,

because it happened in a different goroutine. So the panic will continue to kill the appliaction and it will end as panic.

<strong>recover will only catch panics if they happen on the same routine!</strong>

## More about recover

Only one stack frame must be separated from the stack frame with the exception, 

so that the recover function can capture the exception normally. 

In other words, the recover function catches the exception of the stack frame of the calling function at the grandfather level 

(just enough to cross one level of defer function)！See the example below:

````go
func main() {
		defer func() {
			if r := MyRecover(); r != nil {
					../
			}
		}()
		
  	panic(1)
}

func MyRecover() interface{} {
  	return recover()
}
````

It won't recover the panic and regain the control of flow, but the next example can work:

````go
func main() {
		defer MyRecover()
		
  	panic(1)
}

func MyRecover() interface{} {
  	return recover()
}
````

In fact, the recover function call has stricter requirements: we must call recover directly in the defer function. 

If defer calls the wrapper function of the recover function, the exception capture will fail.