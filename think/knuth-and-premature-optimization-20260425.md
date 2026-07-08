# Donald Knuth 与 "Premature Optimization"

> 来源：个人思考 / 与 Claude 的对话整理
> 收录时间：2026-04-25 CST

---

## 一、Donald Knuth（高德纳）

### 生平

- 1938 年生于美国威斯康星州密尔沃基，德裔美国人
- 1960 年凯斯理工学院数学学士（同时被授予硕士学位，因表现过于突出）
- 1963 年加州理工学院数学博士
- 1968 年起任教于斯坦福大学计算机系，1993 年正式退休（荣休教授）
- 中文名「高德纳」由姚期智的夫人储枫所取

### 主要贡献

#### 1. 《计算机程序设计艺术》（The Art of Computer Programming, TAOCP）

- 自 1962 年开始撰写，原计划 7 卷，目前已出版第 1–4A 卷
- 被誉为算法领域的「圣经」
- 比尔·盖茨曾说：「如果你认为你是一名真正优秀的程序员，请阅读 Knuth 的《计算机程序设计艺术》……如果你能读完整套书，请把简历发给我。」
- 书中所有代码用他自创的 **MIX / MMIX 汇编语言**编写

#### 2. TeX 与 METAFONT

- 1970 年代末因不满 TAOCP 第二版的排版质量，亲自开发了排版系统 **TeX**
- 配套设计了字体系统 METAFONT 和 Computer Modern 字体族
- 至今仍是学术界（尤其是数学、物理、CS）论文排版的事实标准（LaTeX 即基于 TeX）
- 「捐献」给公众领域，不收版税

#### 3. 算法分析（Analysis of Algorithms）

- 开创了「算法分析」这门学科，引入 **大 O 渐近记号**的现代用法
- Knuth-Morris-Pratt（KMP）字符串匹配算法
- Knuth-Bendix 完备化算法
- LR(k) 解析、属性文法等编译原理基础

#### 4. 文学化编程（Literate Programming）

- 主张「程序首先是写给人看的，其次才是给机器执行」
- 开发 WEB / CWEB 系统，将代码与文档交织书写

### 性格与轶事

- **悬赏支票**：在书中发现错误者可获得 2.56 美元（"hexadecimal dollar"，$2^8$ 美分）的支票，许多人裱起来从不兑现
- **拒绝邮件**：1990 年起停用电子邮件，认为 email「适合需要在生活底层的人」，他要做的是「在生活顶层」深度思考
- **版本号**：TeX 版本号趋近于 π（当前 3.141592653…），METAFONT 趋近于 e；他去世后版本号将分别冻结为 π 和 e，并将所有剩余 bug 视为特性
- **图灵奖**：1974 年获得，时年 36 岁，是最年轻的图灵奖得主之一
- 还获得过冯·诺依曼奖、京都奖、美国国家科学奖章等

---

## 二、"Premature Optimization is the root of all evil"

### 出处

出自 Knuth 1974 年发表于 *ACM Computing Surveys* 的论文 **《Structured Programming with go to Statements》**（页 268）。

### 原文（完整版）

> "Programmers waste enormous amounts of time thinking about, or worrying about, the speed of noncritical parts of their programs, and these attempts at efficiency actually have a strong negative impact when debugging and maintenance are considered. **We should forget about small efficiencies, say about 97% of the time: premature optimization is the root of all evil.** Yet we should not pass up our opportunities in that critical 3%."

### 常被忽略的下半句

完整论点是「**but not in that critical 3%**」——Knuth 强调的不是「不要优化」，而是：

1. **97% 的代码不值得为性能牺牲可读性**
2. **3% 的关键路径必须认真优化**
3. 判断「哪 3% 是关键路径」需要**测量**（profiling），不能靠猜

### 常见误读

- ❌ "永远不要优化" / "性能不重要"
- ❌ 用这句话作为写低效代码的挡箭牌
- ✅ Knuth 真正反对的是：**在没有数据支持的情况下，对非热点代码做微优化**

### 归属争议

Knuth 本人多次表示这句话其实**最早可能源自 Tony Hoare**（快速排序、CSP 的作者），他只是引用并发扬了它。Hoare 后来也说自己不记得是否说过，两人互相谦让出处归属。

### 现代回响

- DDIA 作者 Martin Kleppmann、Go 团队的 Rob Pike 等都常引用
- Rob Pike 的《编程五律》第一条：**"You can't tell where a program is going to spend its time. Bottlenecks occur in surprising places, so don't try to second guess and put in a speed hack until you've proven that's where the bottleneck is."**

---

## 附录：自我思考

### 1. 一句话被误用的命运

「Premature optimization is the root of all evil」是被引用最多、也被误用最多的一句话之一。它常常被截断成前半句，用来为「写得糙一点没关系」开脱。但 Knuth 的本意是一种**纪律**：

> 在没有 profiler 数据之前，不要做出关于性能的判断。

这其实是一个很**反直觉**的工程原则——它要求你**承认自己的无知**，并把「优化」从一种凭经验、凭审美的活动，变成一种基于测量的科学活动。

### 2. 「正确 → 清晰 → 快」的层级

可以把它整理成一个朴素的优先级：

1. **Correct**：先让它正确
2. **Clear**：再让它清晰（可读、可维护）
3. **Fast**：用 profiler 找到真正的瓶颈，只对那 3% 做优化

绝大多数性能问题不在循环体里多了一次乘法，而在算法选择、I/O 模式、数据结构这些**结构性**层面。微优化救不了一个算法复杂度错的系统。

### 3. 与「过度工程」的同构

「Premature optimization」其实是一个更普遍模式的特例：

- premature optimization（过早优化）
- premature abstraction（过早抽象）
- premature generalization（过早泛化）
- premature scaling（过早扩展）

它们的共同病灶是：**在缺乏真实信号之前，就为想象中的需求付出真实的复杂度成本**。Knuth 那句话之所以经典，是因为它击中的不只是性能问题，而是一种工程心态。

### 4. 对 AI 时代的启示

在 LLM 辅助编程时代，「过早优化」有了新的形态：让 AI 一上来就生成「企业级」「可扩展」「面向未来」的代码骨架。结果是：还没验证需求是否成立，就背上了一堆抽象层和接口。

Knuth 的告诫在今天依然成立——**先做对最小的事，再用数据决定下一步**。

