### System call tracing

1. 通过声明在 `syscall.h` 中的系统调用数来确定 `trace` 的对象，所以需要一个参数；

2. 按照惯例在 Makefile 文件中 向 `UPROGS` 添加 `$U/_trace`；

3. 在 `make qemu` 之前，需要先将函数原型添加到 `user/user.h` 中：`int trace(int);`

   以及 <strong>stub</strong> 在 `user/usys.pl` 文件中：`entry("trace");` 

   正如第一步，因此需要在 `kernel/syscall.h` 中按顺序添加系统调用数；

4. 经过上述几步可以正常运行 `make qemu`，但是无法在 `sh` 中运行 `trace`，

   这是因为仅仅在用户态下声明了一个功能，并未具体地实现内核中的系统调用；

5. 在 `kernel/proc.c` 中添加 `sys_trace` 函数以及其具体实现，

   因为 `trace` 需要打印当前的调用函数，所以需要在当前进程中记录这个调用函数，

   实验也给出了提示：新增一个变量 trace mask 在 `proc` 结构体中；

6. 函数检索来自用户态的系统调用参数在 `kernel/syscall.c` 文件中；

7. 修改 `fork` 函数将 trace mask 变量从父进程复制到子进程，以满足所有子进程都可以 trace 的需求；

8. 修改 `syscall` 函数去按要求打印 trace，在此之前添加一个存储系统调用名字的数组。

<br>

整体思路是来自实验的提示信息，但是具体地实现因为不熟悉，可能会无从下手，

具体的实现参考代码，如果对系统调用的流程不熟悉，可以慢慢阅读 <strong>4.3 Code: Calling system calls</strong> 章节。

-----

### Sysinfo

