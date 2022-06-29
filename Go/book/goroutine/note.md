# goroutine 和通道

> 原本已经上传过书本上的代码示例，但是一些地方容易忘记，便重新阅读并且写下这篇笔记，增加印象，
>
> 不再去阐述代码的实现，偏向梳理层层递进的增强逻辑以及代码更新的思路。

### 8.1 goroutine

在 Go 里，每一个并发执行的活动称为 `goroutine`，可以假设 `goroutine` 类似于线程。

关于进程、线程、协程的概念，推荐博客：https://juejin.cn/post/6975852498393235487，

（这只是概念上的区别和联系，相关具体机制目前也不太了解，以后学习过程中补充）

当一个程序启动时，只有一个 `goroutine` 来调用 `main` 函数，称它为 `主goroutine`，新的 `goroutine` 通过 `go` 语句进行创建。

语法上，一个 `go` 语句是在普通的函数或者方法调用前加上 `go` 关键字前缀，`go` 语句本身的执行立即完成。

从 `main` 函数返回，当它发生时，所有的 `goroutine` 都暴力地直接终结，然后程序退出，除了从 `main` 返回或者退出程序之外，

没有程序化的方法让一个 `goroutine` 来停止另一个，但是有办法和 `goroutine` 通信来要求它自己停止。

-----

### 8.2 示例：并发时钟服务器

`clock1.go` 是顺序时钟服务器，它以每秒钟一次的频率向客户端发送当前时间，为了连接到服务器，实现简单版本的 `netcat1.go`，

这个程序从网络连接中读取，然后写到标准输出，直到到达 EOF 或者出错，但是第二个客户端必须等到第一个结束才能正常工作，

因为服务器是顺序的，一次只能处理一个客户请求，为此只需要在 `handleConn` 方法前添加 `go` 即可，`clock2.go` 并发处理请求。

----

### 8.3 示例：并发回声服务器

时钟服务器中每一个连接都会起一个 `goroutine`，但是只是达到一个只读的响应，而 echo 服务器示例将会是读写操作，

`reverb1.go` 模拟回声，为了达到接收内容的功能，需要升级客户端程序，得到了 `netcat2.go`，

这样就可以发送终端的输入到服务器，并把服务端的返回输出到终端上，得到了一个简易的并发 Echo 服务器，

但是为了更好地模拟真实世界的回声，需要更多的 `goroutine` 来处理，因此得到升级后的 `reverb2.go`。

然而在使用 `go` 关键词的同时，需要慎重地考虑并发地调用时是否安全，事实上对于大多数类型也确实不安全，以后讨论学习。

-----

### 8.4 Channel

如果说 `goroutine` 是 `Go` 的并发体，那么 `channel` 则是他们之间的通信机制，每个 `channel` 有自己的类型，

可以将其看成数据结构类似于 `map`，因此也是引用传递，其零值为 `nil`，判断是否相同类型可以通过 `==` 比较。

-----

#### 8.4.1 不带缓存的 Channel

基于无缓存 `channel` 的发送和接收操作将导致两个 `goroutine` 做一次同步操作，因此，也被称为同步通道。

当我们讨论两个事件是否并发时，并不是意味着两个事件一定是同步发生，不能确定两个事件之间的先后顺序。

在 8.3 中的客户端程序，当 `主goroutine` 中关闭标准输入时，后台 `goroutine` 可能依然在工作，

需要等待后台 `goroutine` 完成工作后再退出，使用 `channel` 来同步两个 `goroutine`，得到最终版的 `netcat3.go`。

> 基于 channel 发送消息有两个重要方面。首先每个消息都有一个值，但是有时候通讯的事实和发生的时刻也同样重要。
>
> 当我们更希望强调通讯发生的时刻时，我们将它称为**消息事件**。有些消息事件并不携带额外的信息，
>
> 它仅仅是用作两个goroutine之间的同步，这时候我们可以用`struct{}`空结构体作为channels元素的类型，
>
> 虽然也可以使用bool或int类型实现同样的功能，`done <- 1`语句也比`done <- struct{}{}`更短。

#### 8.4.2 串联的 Channels

> Channels也可以用于将多个goroutine链接在一起，一个Channels的输出作为下一个Channels的输入。
>
> 这种串联的Channels就是所谓的管道（pipeline）

通过 `pipeline1.go` 解释这种方式，发送方停止发送后，当一个 `channel` 被关闭后，再向该 `channel` 发送数据将导致 `panic` 异常。

当一个被关闭的 `channel` 中已经发送的数据都被成功接收后，后续的接收操作将不再阻塞，它们会立即返回一个零值。

`Go` 中 `range` 循环可直接在 `channels` 上面迭代。使用 `range` 循环是上面处理模式的简洁语法，

它依次从 `channel` 接收数据，当 `channel` 被关闭并且没有值可接收时跳出循环，结合此方式得到 `pipeline2.go`

#### 8.4.3 单方向的 Channel

当一个 `channel` 作为一个函数参数是，它一般总是被专门用于只发送或者只接收。

为了表明这种意图并防止被滥用，`Go` 语言的类型系统提供了单方向的 `channel` 类型，分别用于只发送或只接收的 `channel`。

类型 `chan<- int` 表示一个只发送 `int` 的 `channel`，只能发送不能接收。相反，类型 `<-chan int` 表示一个只接收 `int` 的 `channel`

只能接收不能发送。（箭头 `<-` 和关键字 `chan` 的相对位置表明了 `channel` 的方向。）这种限制将在编译期检测。

结合此方式改进前两次的 `pipeline` 程序，更加规范性的完善 `goroutine` 之间的通信，得到 `pipeline3.go` 程序。

#### 8.4.4 带缓存的 Channel

```go
func mirroredQuery() string {
    responses := make(chan string, 3)
    go func() { responses <- request("asia.gopl.io") }()
    go func() { responses <- request("europe.gopl.io") }()
    go func() { responses <- request("americas.gopl.io") }()
    return <-responses // return the quickest response
}

func request(hostname string) (response string) { /* ... */ }
```

如果我们使用了无缓存的 `channel`，那么两个慢的 `goroutine` 将会因为没有人接收而被永远卡住。这种情况，称为 `goroutine` 泄漏

这是一个 `Bug`，和垃圾变量不同，泄漏的 `goroutine` 并不会被自动回收，因此确保每个不再需要的 `goroutine` 能正常退出是重要的。

关于选择是否带缓存的通道，仅仅从书上只能看到理论，但是具体的使用，需要经验处理。

-----

### 8.5 并行循环

> 此节示例：考虑生成一批图像的缩略图问题，因为处理文件的顺序没有关系，操作独立，像这样由一些完全独立的子问题组成的问题称为高度并行。

1. 在第一个版本的 `thumbnails1.go` 中，是简单的串行逻辑，我们需要做的就是实现该过程的高度并行，并且让其合理化。

2. 如果仅单单的在方法前添加 `go` 关键字，得到 `thumbnails2.go` 这是非常危险的，所有的 `goroutine` 在没有完成前就返回了，因为没有等待它们执行完毕。

3. 改进方法就是修改内层 `goroutine`，通过一个共享的通道发送事件向外层报告结果，因为是利用 slice 的切片长度去处理，所以知道确切个数，

   这样就得到了第三个版本 `thumbnails3.go`，在这个方法中需要注意的细节就是：<strong>显示传递参数，内部匿名函数获取循环变量的问题，</strong>

   range 过程中 f 的值被所有的匿名函数值共享并且被后续的迭代所更新，导致所有的 `goroutine` 得到的是 slice 的最后元素。

4. 第三个版本看起来似乎已经很不错了，但是缺少错误处理，需要内层 `goroutine` 向外层返回一个错误，得到 `thumbnails4.go`，需要注意下面的错误：

   ```go
   for range filenames {
   	if err := <- errors; err != nil {
   		return err // 注意：不正确，goroutine 泄漏
   	}
   }
   ```

   当一个 error 被接受后，就没有 `goroutine` 继续从 errors 返回通道上进行接收，直至读完，这是非常危险的行为，

   每一个现存的工作 `goroutine` 在试图发送值到此通道的时候永久阻塞，永不终止，这种情况下的 <strong>`goroutine` 泄漏</strong>可能导致整个程序卡住或者系统内存耗尽。

   通常有两个解决方案：

   * 最简单的方案就是用一个有足够容量的缓冲通道，避免阻塞
   * 另一个方案就是在返回第一个 error 的同时，创建另一个 `goroutine` 来读完通道，也就是完成处理

5. `thumbnail5.go` 就是采用缓冲通道来处理这个错误

6. 最终的版本 `thumbnail6.go` 返回新文件所占用的总字节数，并且采用 `sync.WaitGroup` 来进行协程间的计数处理，以便确保所有协程完毕后，再操作。

   这个版本的代码尽管简短，但是如此精巧，我不得不附上代码，以便后续的整理更加清晰：

   ````go
   func makeThumbnails6(filenames <-chan string) int64 {
   	sizes := make(chan int64)
   	var wg sync.WaitGroup // 工作 goroutine 的个数
   	for f := range filenames {
   		wg.Add(1)
   		// worker
   		go func(f string) {
   			defer wg.Done()
   			thumb, err := thumbnail.ImageFile(f)
   			if err != nil {
   				log.Println(err)
   				return
   			}
   			info, _ := os.Stat(thumb) // 可以忽略错误
   			sizes <- info.Size()
   		}(f)
   	}
   	
   	// closer
   	go func() {
   		wg.Wait()
   		close(sizes)
   	}()
   	
   	var total int64
   	for size := range sizes {
   		total += size
   	}
   	return total
   }
   ````

   需要注意 Add 和 Done 方法的不对称性：

   * Add 递增计数器，它必须在工作 `goroutine` 开始之前执行，而不是在中间
   * 另一方面，不能保证 Add 会在关闭者 `goroutine` 调用 Wait 之前发生
   * 另外，Add 有一个参数，但 Done 没有，它等价于 Add(-1)
   * 使用 defer 来确保在发送错误的情况下计数器可以递减

   <strong>在不知道迭代次数的情况下，上面的代码结构是通用的，而 closer 中的等待和关闭必须和 total 并行执行，非常重要。 </strong>

-----

### 8.6 并发的 Web 爬虫

----

