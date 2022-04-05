### LEC 2

----

### Operating system organization

* A key requirement for an operating system is to support several activities at once.

* The operating system must also arrange for isolation between the processes.

* However, it should be possible for processes to intentionally interact.

Thus an operating system must fulfill three requirements: multiplexing, isolation, and interaction.

操作系统必须满足三个要求：多路复用、隔离性、交互性。



There are many ways to achieve these three requirements,

but Xv6 focuses on mainstream designs centered around a monolithic kernel.

Xv6 runs on a multi-core RISC-V microprocessor.

Xv6 is written for the support hardware simulated by qemu's "- machine virt" options.

Xv6 设计采用宏核的方式，并且运行在 RISC-V 微处理器。

----

### Abstracting physical resources

~~Through operating system~~, each application could even have its own library tailored to its needs.

Applications could directly interact with hardware resources and use those resources in the best way for the application.

实际上是每个程序可以通过类似 OS 提供的方法做为库去访问硬件资源，

这样会带来一些问题，所以才引入了 OS 去具体抽象管理硬件资源。

库方法在  cooperative time-sharing scheme 下应用程序没有 bug 并且彼此信任是不错的，

但是过于理想化了，现实往往是不信任且存在许多 bugs，所以相比于 cooperative，更好的方式是 strong isolation



Unix application interact with storage only through the file system' `open` , `read` and `close` system calls,

instead of reading and writing the disk directly.

为了避免 app 直接访问操作一些资源，将资源抽象为服务，这是非常必要的。



Similarly, Unix transparently switched hardware CPUs among processes, 

saving and restoring state as necessary, so that applications don't have to be aware of time sharing.

操作系统调度程序采用的上层策略，一些常见的调度策略 scheduling policy：

* 先进先出 FIFO『First In First Out』
* 最短任务优先 SJF『Shortest Job First』
* 最短完成时间优先 STCF『Shortest Time-to-Completion First』
* 轮转 RR 『Round-Robin』
* 多级反馈队列 MLFQ 『Multi-level Feedback Queue』
* 比例份额 『proportional-share』



Many forms of interaction among Unix processes occur via file descriptors.

文件描述符不仅抽象了许多细节，并且也简化交互方式，比如管道。

----

### User mode, supervisor mode, and system calls

强隔离性需要在应用程序和操作系统之间有一个明显的分界，并且当一个 app 错误时，

不应该导致其他 app 错误，甚至导致 OS 错误，而是 OS 应当维持其他任务的正常运行，并且善后。

为了实现隔离性，OS 必须确保 app 不能修改 OS 的数据指令，以及访问其他进程的内存。



RISC-V has three modes in which the CPU can execute instructions: 

**machine mode:** Instructions have full privilege.

**supervisor mode: **the CPU is allowed to execute privileged instructions.

**user mode:** An application can execute only user-mode instructions and is said to be running in user space.

while the software in supervisor mode can also execute privileged instructions

and is said to be running in kernel space. The software running in kernel space is called the `kernel`.

CPU 提供为强隔离性提供硬件支持，RISC-V 提供三种模式。

程序若仅能执行 U mode 指令，可以说其在 user space 中运行，

软件执行 S mode 指令，运行在 kernel space，可以特指 kernel 内核。



An application that wants to invoke a kernel function must transition to the kernel.

如果一个运行在 U mode 下的 app 想要执行 privileged instruction，CPU 不会执行，

并且切换到 S mode 去终止这个 app，这是非常简单有效的，可以通过系统调用使用功能。



CPUs provide a special instruction that switched the CPU from user mode to supervisor mode

and enters the kernel at an entry point specified by the kernel.

It is important that the kernel control the entry point for transitions to supervisor mode.

RISC-V 提供 `ecall` 指令，一旦切换了 mode，kernel 可以检验系统调用的参数是否正确，

从而决定是否执行，**entry point** 是非常重要的，在配套的实验中可以练习。

----

### Kernel organization

A key design question is what part of the operating system should run in supervisor mode.

One possibility is that the entire operating system resides in the kernel, called a `monolithic kernel`.

一种 OS 设计方式是将整个 OS 都放置在内核中，称为**宏核**。



To reduce the risk of mistakes in the kernel, OS designers can minimize the amount of operating system code that 

runs in supervisor mode, and execute the bulk of the operating system in user mode. 

This kernel organization is called a `microkernel`.

另外一种设计方式是为了减少内核中的错误，最小化在 S mode 中 OS 代码，

并且将一部分 OS 放置在 U mode 中，这种称为**微核**。



Xv6 is implemented as a monolithic kernel, like most Unix operating systems.

Thus, the xv6 kernel interface corresponds to the operating system interface,

and the kernel implements the complete operating system.

---

### Code: Xv6 organization

The xv6 kernel source is in the kernel/ sub-directory. 

The source is divided into files, following a rough notion of modularity.

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/kernel-source.png" alt="system call" style="max-width: 70%">

-----

### Process overview

The unit of isolation is a process.

The process abstraction prevents one process from wrecking or spying on another process's memory, CPU, file descriptors, etc. 

It also prevents a process from wrecking the kernel itself, so that a process can't subvert the kernel's isolation mechanisms.

在 xv6 和其他 Unix 操作系统中，隔离的单元称为进程，进程的抽象方便实现了隔离性。



To help enforce isolation, the process abstraction provides the illusion to a program that it has its own private machine. 

A process provides a program with what appears to be a private memory system and its own CPU to execute.

实际上就是操作系统其中一个特性：虚拟性（CPU 虚拟、内存虚拟），

为了更好地**虚拟化**，操作系统需要一些低级机制 mechanism，也就是一些低级方法或协议，实现所需功能。



**Xv6 use page tables to give each process its own address space which translates a virtual address to a physical address.**

页表是由硬件实现的，操作系统需要硬件的支持以便更好地实现功能。



Xv6 maintains a separate page table for each process that defines that process's address space.

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/page-table.png" alt="system call" style="max-width: 40%">

The xv6 kernel maintains many pieces of state for each process，

which it gathers into a struct proc (`kernel/proc.h:86`).

A process's most important pieces of kernel state are its page table, kernel stack and run state.

一个进程最重要的内核状态是它的页表、内核栈和运行时状态

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/proc.png" alt="system call" style="max-width: 70%">



**Each process has a thread of execution (thread) that executes the process's instructions.**

A thread can be suspended and later resumed, to switch transparently between processes, 

the kernel suspends the currently running thread and resumes another process's thread.

每一个进程有两个栈：用户栈和内核栈，当执行用户指令时，只有它的用户栈被使用，

当进程因为系统调用或者中断进入内核时，内核代码执行在内核栈，

但是用户栈仍然包含保存的数据，只不过没有 actively 使用，

一个进程的线程在主动使用用户栈和内核栈之间交替，

并且内核栈是独立的（不受用户代码影响），以便内核仍可运行即使用户栈被破坏。



