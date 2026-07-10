# Go 语言能力考察题库（四年后端对标 · 白板级）

> 来源：个人思考 + 学习（自我考察题库）
> 收录时间：2026-06-08 17:00 CST
>
> 对标：`CLAUDE.md` 能力地图 **格 1（语言与运行时）**。
> 目的：按「四年后端该掌握」的深度自测 Go，定位薄弱点，再逐块沉淀为原理笔记并进复习调度。

---

## 使用说明

- **只出题、暂不给答案**：先盖住任何资料，自己讲一遍（active recall）。每题后的「**追问**」模拟面试官的三层深挖——能接住追问才算真懂。
- **建议分批作答**：一次答一两个大类，批改后补漏，再决定哪些进 `go-internals-*` 原理笔记。
- **毕业标准**：每一类里 ≥80% 的题能口述讲清 + 扛住追问。未达标的题标记 ❌，进复习表重挖。

---

## A. 语法与类型系统基础

1. Go 是值传递还是引用传递？slice、map、channel 作为参数传进函数，函数内修改会影响外部吗？为什么？**追问**：传 slice 进去 append，外部看得到新元素吗？分情况说。
2. `var s []int` 和 `s := []int{}` 有什么区别？哪个是 nil？对 nil slice 做 append / range / len 会怎样？
3. 数组和 slice 的本质区别？slice 的底层结构（三个字段）是什么？
4. `new(T)` 和 `make(T)` 的区别？make 为什么只能用于 slice/map/channel？
5. Go 的零值机制是什么？哪些类型零值是 nil？一个 struct 的零值长什么样？
6. `iota` 怎么用？写一个用 iota 表示字节单位（KB/MB/GB）的枚举。
7. 常量和变量的区别？Go 有没有真正的 const 指针 / const 切片？
8. `int` 在 32 位和 64 位平台上一样大吗？`rune` 和 `byte` 各是什么的别名？
9. 类型转换（conversion）和类型断言（assertion）的区别？`i.(T)` 和 `i.(type)` 分别用在哪？
10. 字符串能不能修改某个字节？`s[0] = 'x'` 能编译吗？为什么 string 不可变？

## B. slice / map / string 底层

11. slice 的 `len` 和 `cap` 区别？`s := make([]int, 3, 10)` 后 len/cap 各是多少？
12. append 触发扩容的规则是什么？小切片和大切片扩容因子一样吗？（讲机制，不用背死数字）
13. **经典坑**：`a := []int{1,2,3,4,5}; b := a[1:3]`，b 的 len/cap？对 b 做 append 会不会改到 a？什么时候会、什么时候不会？
14. 怎么安全地从 slice 中间删除一个元素？有几种写法，各自的性能 / 陷阱？
15. 两个 slice 共享底层数组时怎么彻底切断？`copy` 和 `append([]T{}, s...)` 哪个更稳？
16. map 的底层结构是什么（bucket/hmap）？为什么遍历 map 是无序的，这是 bug 还是设计？
17. map 能并发读写吗？并发写会发生什么？怎么解决（列出 ≥3 种方案及取舍）？
18. 为什么不能对 map 的 value（struct 类型）直接赋值字段，如 `m["k"].field = 1`？怎么绕？
19. map 的 key 有什么要求？slice 能做 key 吗？struct 能吗？
20. 删除 map 的大量元素后内存会释放给 OS 吗？map 会缩容吗？怎么真正回收？
21. string 和 []byte 互转会发生拷贝吗？有没有零拷贝的办法，代价是什么？
22. `for i, c := range str` 里 c 的类型是什么？遍历中文字符串时 i 是字节索引还是字符索引？

## C. 接口与方法集

23. 接口的底层结构（iface vs eface）是什么？分别存了什么？
24. **经典坑**：一个返回了 `*T`（nil 指针）的函数，赋给 error 接口后，`err != nil` 成立吗？为什么？这就是著名的 nil 接口陷阱，讲清楚。
25. 值接收者 `func (t T)` 和指针接收者 `func (t *T)` 的方法集差异？什么时候 `T` 满足接口、什么时候只有 `*T` 满足？
26. 一个值类型变量能调用指针接收者的方法吗？什么情况下能、什么情况下不能（可寻址性）？
27. 空接口 `interface{}` / `any` 的代价是什么？为什么说滥用 any 会丢失类型安全和性能？
28. 接口的动态派发有性能开销吗？和直接调用比？编译器能内联接口方法吗？
29. 怎么判断一个接口值底层是哪个具体类型？type switch 怎么写？
30. Go 1.18 泛型出现后，哪些原来用 interface{} 的场景该换成泛型？泛型 vs 接口的取舍？
31. 嵌入（embedding）和继承的区别？嵌入接口、嵌入 struct 各自发生什么？方法提升规则？

## D. defer / panic / recover / error

32. defer 的执行顺序？多个 defer 是栈还是队列？
33. **口算题**：`func f() (r int) { defer func(){ r++ }(); return 5 }` 返回几？换成非命名返回值呢？讲清 defer 与返回值的交互。
34. defer 的参数是什么时候求值的？`defer fmt.Println(i)` 在循环里会打印什么？
35. defer 有性能开销吗？Go 各版本对 defer 做过什么优化（开放编码 defer）？
36. recover 只能在哪里生效？在 defer 调用的函数的再下一层调用里 recover 有用吗？
37. 一个 goroutine panic 了，主 goroutine 能 recover 它吗？为什么？这对工程意味着什么？
38. Go 1.13 的 error wrapping（`%w`、`errors.Is`、`errors.As`）解决了什么问题？三者区别？
39. `errors.Is` 和 `==` 比较 error 有什么不同？自定义 error 怎么支持 Is？
40. panic 和 error 的使用边界在哪？什么场景该 panic，什么场景该返回 error？
41. 如何优雅地给整个 HTTP handler 加 panic 恢复中间件？写出核心结构。

## E. goroutine 与并发模型

42. goroutine 和线程的区别？初始栈多大？栈是怎么增长的（连续栈 / 栈拷贝）？
43. 能开多少个 goroutine？百万 goroutine 的主要成本在哪？
44. **经典坑**：`for _, v := range list { go func(){ use(v) }() }` 有什么 bug？（注意 Go 1.22 前后行为变化）怎么修？
45. goroutine 泄漏是怎么发生的？举三种典型场景。怎么检测（pprof/goroutine dump）？
46. 主 goroutine 退出，其它 goroutine 会怎样？怎么等所有 goroutine 结束？
47. 怎么限制并发数（goroutine 池 / 信号量）？用 channel 实现一个信号量。
48. WaitGroup 的 Add/Done/Wait 用错会怎样？Add 放在 goroutine 内部为什么是 bug？
49. `errgroup` 解决了什么 WaitGroup 不能做的事？
50. 并发和并行的区别？`GOMAXPROCS` 控制的是什么？设成 1 还能并发吗？

## F. channel

51. channel 的底层结构（hchan）是什么？有缓冲和无缓冲的区别？
52. 向 nil channel 读 / 写会怎样？向已关闭的 channel 读 / 写各会怎样？
53. 怎么判断 channel 已关闭？`v, ok := <-ch` 里 ok 的含义？
54. **谁负责 close channel**？为什么"接收方不应 close、不应重复 close"？重复 close 会怎样？
55. 无缓冲 channel 的发送和接收谁先阻塞？它保证了什么同步语义（happens-before）？
56. select 的随机性体现在哪？`default` 分支的作用？怎么用 select 实现超时？
57. 用 channel 实现：① 生产者-消费者；② fan-in / fan-out；③ 优雅关闭一组 worker。写出骨架。
58. channel 和 mutex 各自适合什么场景？"Don't communicate by sharing memory" 这句话怎么理解，什么时候反而该用 mutex？
59. 如何用 channel 实现一个并发安全、可超时的对象池？
60. 关闭 channel 来广播"退出信号"为什么比发 N 个值更好？（配合 context 讲）

## G. sync 包与内存模型

61. Mutex 和 RWMutex 的区别？RWMutex 在读多写少下一定更快吗？有什么坑（写饥饿）？
62. Go 的 Mutex 是公平锁吗？讲一下正常模式 vs 饥饿模式。
63. `sync.Once` 怎么保证只执行一次？它的底层（原子 + 锁双检）？
64. `sync.WaitGroup` 能复用吗？Wait 之后再 Add 安全吗？
65. `sync.Pool` 解决什么问题？里面的对象什么时候被回收（和 GC 的关系）？用它要注意什么？
66. `atomic` 包能做什么？`atomic.Value` 适合什么场景？CAS 是什么，ABA 问题在 Go 里需要担心吗？
67. **内存模型**：没有同步原语时，一个 goroutine 写、另一个读同一变量，为什么可能读到旧值甚至撕裂值？happens-before 是什么？
68. data race 和 race condition 是一回事吗？`-race` 检测器能查出哪种？
69. 双重检查锁（DCL）在 Go 里能用裸 if 实现吗？为什么要用 Once 或 atomic？
70. 写一个并发安全的计数器，分别用 mutex 和 atomic，说出性能差异来源。
71. 伪共享（false sharing）是什么？Go 里怎么用 padding 缓解？

## H. context

72. context 解决了什么问题？四个派生函数（WithCancel/WithTimeout/WithDeadline/WithValue）各干嘛？
73. context 的取消信号是怎么传播到子 context 的？底层是 channel 还是回调？
74. **经典坑**：`WithValue` 该传什么、不该传什么？为什么不建议用它传函数参数？key 为什么要用自定义类型？
75. 拿到 ctx 后不调用 cancel 会怎样（资源泄漏）？为什么 `defer cancel()` 几乎是必须的？
76. context 取消后，正在执行的 goroutine 会被强制杀死吗？谁负责响应取消？
77. 一个请求链路里 ctx 怎么贯穿（HTTP → service → DB）？超时是怎么逐层生效的？
78. `ctx.Err()` 的两种返回值分别代表什么？

## I. 调度器（GMP）

79. 完整讲一遍 GMP 模型：G、M、P 各是什么？为什么需要 P 这一层？
80. P 的本地队列和全局队列是什么关系？work-stealing（工作窃取）怎么发生？
81. 一个 goroutine 在什么时机会被调度切换（抢占）？Go 1.14 前的协作式抢占有什么问题，后来怎么改成基于信号的异步抢占？
82. goroutine 执行阻塞的系统调用（如读文件）时，M 和 P 会发生什么（hand off）？这和网络 IO 的处理有何不同？
83. netpoller 是什么？为什么 Go 里写阻塞式的网络代码却不会真的阻塞 OS 线程？
84. `GOMAXPROCS` 默认值？在容器（限了 CPU）里默认值对不对，为什么需要 automaxprocs？
85. `runtime.Gosched()` 做什么？手动让出有意义吗？
86. 一个死循环不含函数调用的 goroutine，在老版本 Go 会卡死整个程序吗？为什么？

## J. 内存管理与 GC

87. Go 的内存分配器分层（mcache/mcentral/mheap、tiny/small/large）大致怎么工作？为什么要分级？
88. Go GC 用的是什么算法（并发三色标记 + 混合写屏障）？讲讲三色不变性。
89. 写屏障（write barrier）解决了什么问题？为什么并发标记需要它？
90. STW（stop-the-world）在 Go 现代 GC 里发生在哪些阶段，大概多久？
91. `GOGC` 控制什么？调大调小分别影响什么？`GOMEMLIMIT`（1.19+）又解决了什么？
92. 怎么判断你的程序有内存泄漏（Go 里"泄漏"通常指什么）？用什么工具定位？
93. GC 的触发时机有哪些？手动 `runtime.GC()` 什么时候才该用？
94. 内存对齐是什么？调整 struct 字段顺序为什么能省内存？给个例子口算 struct 大小。

## K. 逃逸分析与性能

95. 什么是逃逸分析？栈分配和堆分配的性能差异来源？
96. 列举 ≥4 种会导致变量逃逸到堆的典型情况。
97. 返回局部变量的指针，在 Go 里安全吗？和 C 有何不同？为什么？
98. 怎么查看逃逸分析结果（`go build -gcflags`）？怎么读懂输出？
99. `[]byte` 和 `string` 频繁转换的性能问题怎么优化？
100. 预分配（`make([]T, 0, n)`、map 预设容量）为什么重要？不预分配的代价？
101. pprof 能采集哪几类 profile（cpu/heap/goroutine/block/mutex）？各自查什么问题？
102. 怎么用 benchmark + `-benchmem` 定位一个函数的分配热点？`b.ResetTimer` / `b.ReportAllocs` 干嘛？
103. 字符串拼接：`+`、`fmt.Sprintf`、`strings.Builder`、`bytes.Buffer` 的性能排序和原因？

## L. 工程化与工具链

104. Go module 的 `go.mod` / `go.sum` 各是什么？`go.sum` 防的是什么攻击？
105. 语义化版本与最小版本选择（MVS）是什么？`replace`、`exclude` 何时用？
106. `go vet`、`golangci-lint`、`-race`、`-cover` 各自的职责？
107. build tag / 条件编译怎么用？跨平台构建怎么做？
108. `init()` 函数的执行顺序？多个 init、多个包之间的初始化顺序怎么定？
109. 怎么做依赖注入？Go 里为什么不流行重型 DI 框架？
110. 单元测试里怎么 mock 一个外部依赖（接口 + 桩）？table-driven test 怎么写？
111. 怎么优雅关闭一个 HTTP 服务（graceful shutdown）？把 signal + context + server.Shutdown 串起来。

## M. 综合 / 手写题（白板）

112. 手写一个并发安全的 LRU Cache（map + 双向链表 + mutex）。
113. 手写一个令牌桶 / 漏桶限流器。
114. 用两个 goroutine 交替打印 1~100 / 奇偶数（channel 或 mutex+cond 两种解法）。
115. 实现一个带超时和最大并发的批量任务执行器，返回所有结果或首个错误（errgroup 思路）。
116. 实现一个 singleflight（同一 key 的并发请求只穿透一次）。
117. 用 channel 实现一个简单的 worker pool，支持优雅退出和等待已入队任务完成。

---

> 以下 N–Q 四类为补全市场高频盲点（2026-06-08 追加），让 Go 这一科自测闭环。

## N. 反射 reflect

118. 反射三大定律是什么？`reflect.Type` 和 `reflect.Value` 的关系？
119. 怎么用反射读取 / 修改一个 struct 的字段？为什么必须传指针才能 `Set`？讲清可寻址性（`CanSet`）。
120. struct tag 是怎么被解析的？**手写**一个简化版：按 tag 把 `map[string]string` 填进 struct（json.Unmarshal 的雏形）。
121. 反射为什么慢？慢在哪？生产里怎么规避（缓存 Type 信息 / 代码生成）？
122. `encoding/json` 的 Marshal/Unmarshal 底层用反射做了什么？为什么私有（小写）字段不被序列化？

## O. 泛型深入

123. 泛型的类型约束（constraints）怎么写？`any`、`comparable`、自定义约束、type set 各是什么？
124. 约束里的 `~int` 中 `~` 是什么意思？和直接写 `int` 有何区别？
125. Go 泛型底层是单态化（每个类型生成一份代码）还是字典传递（GC shape stenciling）？对性能和二进制体积的影响？
126. 什么场景该用泛型、什么场景该用接口？各举一个「泛型明显优于 interface{}」和「不该用泛型」的例子。
127. 为什么 Go 不支持「泛型方法」（类型参数只能加在类型 / 函数上，不能加在方法上）？这限制了什么设计？

## P. 闭包、方法值、标准库接口

128. 闭包是什么？它捕获的是变量的值还是引用？捕获为什么常导致变量逃逸到堆？
129. 方法值（method value）和方法表达式（method expression）的区别？`t.M` 和 `T.M` 各是什么类型？
130. `io.Reader` / `io.Writer` 为什么是 Go 设计哲学的典范？「小接口 + 组合」好在哪？
131. `io.Copy` 是怎么工作的？它如何利用 `ReaderFrom` / `WriterTo` 走零拷贝快路径？
132. **手写**一个 `io.Reader` 包装器：把底层读到的内容转大写后返回（中间件式 wrapper）。

## Q. 底层、性能与版本特性

133. 内联（inlining）是什么？哪些函数不能被内联？内联和逃逸分析怎么相互影响？怎么查看内联决策？
134. map 扩容的两种情况：**翻倍扩容 vs 等量扩容**分别什么条件触发？**渐进式 rehash**（oldbuckets / evacuate）是怎么做的？
135. `sync.Map` 的底层（read map + dirty map + misses 计数）怎么工作？它适合什么读写模式，什么情况下反而比 `mutex+map` 慢？
136. `unsafe.Pointer` 的四条合法使用规则是什么？`uintptr` 为什么不能长期持有（GC 移动 / 回收）？
137. 用 `unsafe` 做 string ↔ []byte 零拷贝转换怎么写？有什么风险（只读内存段、GC）？
138. 列一条你用得上的 Go 版本特性时间线：1.18 泛型 / 1.20 `errors.Join` / 1.21 `min·max·clear·slog` / 1.22 loopvar 修复——各解决了什么痛点？
139. Go 1.22 之前 `for` 循环变量复用导致的经典 bug，1.22 怎么改的？这个改动有没有兼容性风险？

---

## 答题进度追踪

> 答完一类就更新；❌ 标记的题进复习重挖。

| 大类 | 主题 | 状态 | 薄弱点（待补） |
|:--:|------|:--:|------|
| A | 语法与类型系统 | 未答 | — |
| B | slice/map/string 底层 | 未答 | — |
| C | 接口与方法集 | 未答 | — |
| D | defer/panic/recover/error | 未答 | — |
| E | goroutine 与并发模型 | 未答 | — |
| F | channel | 未答 | — |
| G | sync 与内存模型 | 未答 | — |
| H | context | 未答 | — |
| I | 调度器 GMP | 未答 | — |
| J | 内存管理与 GC | 未答 | — |
| K | 逃逸分析与性能 | 未答 | — |
| L | 工程化与工具链 | 未答 | — |
| M | 综合 / 手写题 | 未答 | — |
| N | 反射 reflect | 未答 | — |
| O | 泛型深入 | 未答 | — |
| P | 闭包 / 方法值 / io 接口 | 未答 | — |
| Q | 底层 / 性能 / 版本特性 | 未答 | — |

---

## 附录：自我思考

- 这份题库是「考察」而非「学习」——先暴露我「会用不会讲」的具体边界，再针对薄弱类做原理深挖，比从头通读教程更省力、更对焦面试。
- 题目刻意安排了**手感区**（A/E/F：天天写）和**盲区**（I/J：调度器和 GC 平时碰不到底层）。预期手感区也会在「追问」层翻车——这正是要验证的：用得熟 ≠ 讲得清。
- 沉淀计划：每类答完后，把答错 / 答不全的题连同正确原理，单独成篇 `go-internals-{子主题}-YYYYMMDD.md`（如 `go-internals-gmp-scheduler-*`、`go-internals-gc-*`），并把本表的 ❌ 项挂上去做复习入口。
