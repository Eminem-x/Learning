# TERM 变量与 terminfo 数据库：为什么 SSH/multipass 进远程机后 clear 报 "unknown terminal type"

> 来源：个人思考（终端实操踩坑 + 原理梳理）
> 收录时间：2026-05-29 14:36 CST

---

## 一、现象

用 Ghostty 终端，通过 `multipass shell` 或 `ssh` 进入远程 Ubuntu 后，运行 `clear` 报错：

```
ubuntu@ip-10-101-135-224:~$ clear
'xterm-ghostty': unknown terminal type.
ubuntu@ip-10-101-135-224:~$ echo $TERM
xterm-ghostty
```

- multipass(默认 Ubuntu 22.04)、AWS EC2(Ubuntu 24.04)上**行为完全一致**。
- 报错的关键词是 **terminal type**，不是 `command not found`——`clear` 命令是存在的，是它**查不到当前终端的"说明书"**。

## 二、核心心智模型：终端是两层

```
[终端模拟器]  ←→  [里面跑的程序]
 Ghostty/iTerm        bash, clear, vim, top
 负责"画像素"          只会往 stdout 吐字节
```

关键事实：**程序本身不会画屏幕**。`clear`/`vim`/`top` 能做的只有"往输出流写字节"。真正执行清屏、移动光标、上色的是**终端模拟器**。

程序怎么命令终端清屏？发一串特殊字节，叫**控制序列 / 转义序列(escape sequence)**：

```
清屏的经典序列：ESC [ 2 J   →   实际字节 \x1b[2J
```

终端模拟器读到 `\x1b[`(ESC + `[`)就知道"后面不是要显示的文字，是给我的命令"，于是执行。

## 三、问题根源：不同终端的控制码不一样

历史上终端是**实体硬件**(DEC VT100、VT220、Wyse 60、ADM-3A…)，各家的清屏码、光标码、颜色码都不同：

- VT100 清屏 `ESC[2J`
- 某些老终端清屏是 `ESC[H ESC[J` 或一个 `^L`
- 有的终端根本不支持颜色

如果程序把控制码写死，换一种终端就全乱。

## 四、解决方案：terminfo 数据库（一本"终端说明书"字典）

Unix 的解法：**别把控制码写死在程序里，查字典**。这本字典就是 **terminfo**(早期叫 termcap)，通常在：

```
/usr/share/terminfo/
  ├── v/vt100
  ├── x/xterm
  ├── x/xterm-256color
  └── ...
```

每个文件描述**一种终端型号**，是一组能力(capability)键值对：

| 能力名 | 含义 | 值(控制码) |
|--------|------|------------|
| `clear` | 清屏 | `\E[H\E[2J` |
| `cup`   | 光标定位 | `\E[%i%p1%d;%p2%dH` |
| `colors`| 支持几种颜色 | `256` |
| `cols`  | 屏幕宽度 | 80 |

所以 `clear` 真正干的事：

```
1. 看环境变量 TERM，知道"我是什么终端" → 比如 xterm-256color
2. 去 terminfo 翻开 xterm-256color 那一页
3. 找到 clear 能力对应的字节串
4. 写到 stdout
5. 终端模拟器收到 → 清屏
```

## 五、TERM 变量：那张"我是谁"的身份证

`TERM` 的值就是"当前终端在 terminfo 字典里的页码名"：

```bash
echo $TERM     # Ghostty 里 → xterm-ghostty
```

- **谁设置它**：终端模拟器自己。Ghostty 启动 shell 时把 `TERM=xterm-ghostty` 塞进环境；iTerm2 设 `xterm-256color`；tmux 设 `screen`/`tmux-256color`；Linux 文本控制台是 `linux`。
- **`xterm-ghostty` 是 Ghostty 这个应用自己的规范名**，所以在本机 Ghostty 里一切正常(本机字典有这页)。

## 六、出错的完整链条

```
Ghostty (TERM=xterm-ghostty)
   └─ ssh / multipass shell 连接，申请交互 shell
        └─ 把本机 $TERM 原封不动带进远端 PTY
             └─ 远端 clear 读到 TERM=xterm-ghostty
                  └─ 去远端 /usr/share/terminfo 找 "xterm-ghostty"
                       └─ 找不到（ncurses 太老没收录）
                            └─ 报错 unknown terminal type
```

> ⚠️ 不是"找不到 clear 命令"，是 **clear 跑起来了但查字典没这页**，不知道发什么码，只好罢工。

## 七、最关键的地基：程序在哪跑？谁在渲染？（破解"终端不是 Mac 自己的吗"的困惑）

一个常见误会：「终端是我 Mac 上的 Ghostty，我只是 ssh 连到服务器，clear 的不也是 Ghostty 吗，为什么跟服务器有关？」

破解点在于分清**两个角色**：**谁在运行程序** vs **谁在显示画面**。SSH 之后这两者被拆到了两台机器上。

```
   你的 Mac                          远程服务器
┌──────────────┐                ┌──────────────────┐
│   Ghostty    │ ←─── SSH ───→  │  bash, clear,    │
│ (终端模拟器)  │   只传字节流    │  vim, top ...    │
│              │                │  这些程序都在     │
│ 只做两件事:   │                │  这台跑!         │
│ 1.把键盘敲击  │                │                  │
│   发过去      │                │  /usr/share/     │
│ 2.把收到的    │                │   terminfo/      │
│   字节画出来  │                │  ← 字典在这台!   │
└──────────────┘                └──────────────────┘
```

当你敲 `clear` 回车：

1. Ghostty 把 `c l e a r ⏎` 这几个**按键字节**通过 SSH 发到服务器。
2. **服务器上的 bash** 收到，启动**服务器上的 `/usr/bin/clear`**。
3. clear 要查字典，它查的是**服务器自己的 `/usr/share/terminfo/`**（因为它就跑在服务器上，只能看到服务器的文件系统）。
4. 服务器字典里没有 ghostty 这页 → clear 报错，**这串报错文字**再通过 SSH 传回 Ghostty 显示。

**所以报错是服务器上的 clear 喊出来的，Ghostty 只是个传声筒 + 显示器。**

### 一个易错点：谁"渲染"？

「服务器渲染了 clear」这个说法差一点。准确说：

> 服务器**没有渲染**，它只是**发了一条清屏命令（控制码 `\x1b[2J`）**；真正把屏幕擦干净的**渲染动作，仍然是 Ghostty 在 Mac 本地做的**。

| 步骤 | 在哪台机器 | 干了什么 |
|---|---|---|
| 决定"要清屏" + 发出 `\x1b[2J` | **服务器**(clear 程序) | 只吐了 4 个字节 |
| 收到 `\x1b[2J` → 真的把像素擦掉 | **Mac**(Ghostty) | 实际渲染 |

服务器从头到尾没碰过像素，它连你屏幕长什么样都不知道。它只会吐字节：普通文字（直接显示）或控制码（命令 Ghostty 干活）。

### 心智模型：服务器是"导演"，Ghostty 是"舞台"

- **服务器(大脑)**：喊指令——"打一行字""清屏""这段标红"，以字节流发出。
- **Ghostty(舞台/屏幕)**：听指令、真正执行——真正画字、清屏、上色。

> 「把终端想象成服务器的远程屏幕 + 键盘」这个模型是**对的**。只需记住分工：**服务器决定"显示什么"（发码），Ghostty 负责"真正画出来"（渲染）**。

用这个模型再看报错：导演（服务器的 clear）想喊"清屏"，得先翻**自己手里的手册（服务器的 terminfo）**确认 ghostty 牌舞台的清屏指令怎么写；手册没这页 → 导演**连指令都没喊出来就卡住报错**。舞台（Ghostty）再能干，也没收到任何指令。这也解释了为什么 `Ctrl+L` 永远好使：它由 Mac 本地的 readline 直接发标准码，**根本不经过"服务器查字典"这条链路**。

### 回到三个疑问

| 疑问 | 真相 |
|---|---|
| "终端不是 Mac 自己的吗?" | 终端**模拟器**(画面)是 Mac 的；但 `clear` **程序**跑在服务器上 |
| "clear 的不也是 ghostty 终端吗?" | clear 不"清"终端，它只**发一串码**让终端去清；发码的决策在服务器 |
| "为什么跟目标服务器有关?" | 查字典的是**服务器上的 clear**，查的是**服务器上的 terminfo**，Mac 的字典它根本看不到 |

## 八、关键追问：TERM 在哪一步被注入？multipass 和 SSH 有何区别？

**结论：`multipass shell` 本质就是一次 SSH 连接**(multipassd 在 VM 里跑了 sshd)。两者**注入机制完全相同**，没有本质区别。

注入发生在 **SSH 协议握手时申请伪终端的那一步——`pty-req`**：

```
你的 Mac (Ghostty, TERM=xterm-ghostty)
  │  客户端要开交互 shell → 申请 PTY
  └──► 发送 pty-req 通道请求，内容携带:
          TERM = "xterm-ghostty"   ← 直接读本机 $TERM 填入
          窗口大小 = 80x24
          │
          ▼  远端 sshd 在新 shell 会话里 export TERM=xterm-ghostty
```

**反直觉细节：TERM 不走 SendEnv/AcceptEnv**

| 普通环境变量(LANG、LC_*) | TERM |
|---|---|
| 走 `env` 请求，需双方配 `SendEnv`/`AcceptEnv` 才放行，默认大多不转发 | 走 **`pty-req`**，只要分配了 PTY 就**无条件携带**，无需任何配置 |

所以只要开的是交互式 shell(有 PTY)，TERM 几乎总会被带到远端——躲不掉。

> **什么时候 TERM 不会被注入？** 没有分配 PTY 时：
> ```bash
> ssh server 'echo $TERM'   # 不带 -t 跑命令，无 PTY → TERM 为空或 dumb
> ssh -T server             # 显式不要 PTY
> ```
> 而 `ssh server`(纯登录)、`ssh -t`、`multipass shell` 都分配 PTY，都会注入。

## 九、那"为什么有的服务器不报错"——决定因素是字典，不是连接方式

`xterm-ghostty` 这一页随 **ncurses 6.4(2023 年)** 才进入官方 terminfo。于是：

- 较新系统(Ubuntu 24.04、较新 Fedora/Arch，ncurses ≥ 6.4)**可能**自带这页 → 不报错。
- 较老镜像(Ubuntu 22.04，ncurses 6.3)→ 没这页 → 报错。

> 本次实测：multipass(22.04) 与 AWS(24.04) **都报错**，说明这两台的 terminfo 里都没收录 ghostty。同样被注入 `xterm-ghostty`，查得到就正常、查不到就报错——**变量是"目标机器够不够新"，不是连接方式。**

之前 ssh 某些服务器不报错，无非两种：① 那台够新自带了；② 当时还没换 Ghostty(老终端 TERM=xterm-256color，这页人人都有)。

| 维度 | `ssh 远程服务器` | `multipass shell` |
|---|---|---|
| 底层协议 | SSH | SSH(multipassd 内置 sshd) |
| TERM 怎么注入 | pty-req 携带本机 $TERM | **完全相同** |
| 是否需要 AcceptEnv | 不需要(走 pty-req) | 不需要 |
| 报不报错的真正决定因素 | **目标机器 terminfo 有没有 xterm-ghostty** | 同左 |

## 十、修法（三档）

```bash
# 方案 A：临时——告诉程序"就当我是标准 xterm"（这页每台机器都有）
export TERM=xterm-256color
clear        # 现在好使了

# 方案 B：永久——写进远端 ~/.bashrc
echo 'export TERM=xterm-256color' >> ~/.bashrc
```

> 方案 B 隐患：以后从真支持 ghostty 的环境进来也会被强行降级。更稳妥的写法是"查不到才降级"：
> ```bash
> # ~/.bashrc
> if ! infocmp "$TERM" >/dev/null 2>&1; then
>     export TERM=xterm-256color
> fi
> ```

```bash
# 方案 C：治本——把 Ghostty 的说明书装进远程机
# 在本机 Mac 上执行：反编译本机 ghostty 条目，通过 SSH 在远端编译进字典
infocmp -x xterm-ghostty | ssh ubuntu@HOST 'tic -x -'
```
- `infocmp -x xterm-ghostty`：把本机字典里 ghostty 这页反编译成文本
- `tic -x -`：在远端把这段文本编译进 terminfo 数据库
- 装完远端就认识 `xterm-ghostty`，无需再降级。

**为什么 xterm-256color 一定有效**：现代终端在控制码层面都兼容 xterm，Ghostty 也只是给 xterm 方言起了新名字；而 `xterm-256color` 这页每台 Unix/Linux 都自带。代价仅是用不到 Ghostty 独有的高级特性——日常 clear/vim/top 用不到。

**为什么 `Ctrl+L` 任何时候都能清屏**：它是 bash/readline 的内建快捷键，直接发标准码，**完全绕过 terminfo 查询**，所以 TERM 再乱它也照清。

## 十一、实用排查命令（一辈子受用）

```bash
echo $TERM                 # 我现在自称是什么终端
infocmp                    # 打印当前 TERM 的完整 terminfo 条目（查得到=没问题）
infocmp xterm-ghostty      # 查某个具体终端是否存在 / 看它的能力表
toe | grep -i ghostty      # 列出本机所有终端类型，看有没有 ghostty
tic -V                     # terminfo 编译器版本（间接看 ncurses 新旧）
tput clear                 # 用 terminfo 查出的清屏码清屏（clear 的本质）
tput colors                # 当前终端支持几种颜色
```

`tput` 是通用工具：查 terminfo 字典并吐出对应控制码；`clear` 本质就是 `tput clear` 的特例。

---

## 附录：自我思考

1. **"命令报错"要先读报错的语义层级**。`unknown terminal type` 三个词就指明了问题在"终端类型/字典查询"层，而非"命令缺失"层。养成"先精读报错关键词、再定位是哪一层出问题"的习惯，比直接搜报错文本更快收敛。

2. **凡是"远程后某个全屏程序行为异常"，先 `echo $TERM` + `infocmp`**。这套故障几乎都是"新终端 + 旧机器没收录"。把 `TERM 变量 + terminfo 字典` 这对概念记牢，vim 花屏、退格键失灵、颜色不对、`top` 错位，往往都是同一个根因的不同表现。

3. **"机制相同、结果不同"的排查思维**。multipass vs SSH 这个困惑的破解点在于：先确认两条路径的底层机制是否真的不同(都是 SSH + pty-req)，再把差异隔离到唯一变量(目标机的 terminfo 版本)。遇到"为什么 A 行 B 不行"时，先找出真正不同的那一个变量，而不是假设整条链路都不同。

4. **治本 vs 治标的取舍**。方案 A/B 是"我适配你"，方案 C 是"让环境认识我"。频繁登录的固定机器值得用 C；偶尔一登的临时机器用 A 足矣。工程上很多兼容性问题都有这组对应选择。
