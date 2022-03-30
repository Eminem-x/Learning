### LEC 2

----

### Operating system organization

* A key requirement for an operating system is to support several activities at once.

* The operating system must also arrange for isolation between the processes.

* However, it should be possible for processes to intentionally interact.

Thus an operating system must fulfill three requirements: multiplexing, isolation, and interaction.

There are many ways to achieve these three requirements,

but Xv6 focuses on mainstream designs centered around a monolithic kernel.

Xv6 runs on a multi-core RISC-V microprocessor.

Xv6 is written for the support hardware simulated by qemu's "- machine virt" options.

----

### Abstracting physical resources

Through operating system, each application could even have its own library tailored to its needs.

Applications could directly interact with hardware resources and use those resources in the best way for the application.

Unix application interact with storage only through the file system' `open` , `read` and `close` system calls,

instead of reading and writing the disk directly.

Similarly, Unix transparently switched hardware CPUs among processes, 

saving and restoring state as necessary, so that applications don't have to be aware of time sharing.

Many forms of interaction among Unix processes occur via file descriptors.

----

### User mode, supervisor mode, and system calls

RISC-V has three modes in which the CPU can execute instructions: 

**machine mode:** Instructions have full privilege.

**supervisor mode: **the CPU is allowed to execute privileged instructions.

**user mode:** An application can execute only user-mode instructions and is said to be running in user space.

while the software in supervisor mode can also execute privileged instructions

and is said to be running in kernel space. The software running in kernel space is called the `kernel`.

An application that wants to invoke a kernel function must transition to the kernel.

CPUs provide a special instruction that switched the CPU from user mode to supervisor mode

and enters the kernel at an entry point specified by the kernel.

It is important that the kernel control the entry point for transitions to supervisor mode.

----

### Kernel organization

A key design question is what part of the operating system should run in supervisor mode.

One possibility is that the entire operating system resides in the kernel, called a `monolithic kernel`.

To reduce the risk of mistakes in the kernel, OS designers can minimize the amount of operating system code that 

runs in supervisor mode, and execute the bulk of the operating system in user mode. 

This kernel organization is called a `microkernel`.

Xv6 is implemented as a monolithic kernel, like most Unix operating systems.

Thus, the xv6 kernel interface corresponds to the operating system interface,

and the kernel implements the complete operating system.

---

### Process overview

The unit of isolation is a process.

The process abstraction prevents one process from wrecking or spying on another process's memory, CPU, file descriptors, etc. 

It also prevents a process from wrecking the kernel itself, so that a process can't subvert the kernel's isolation mechanisms.

To help enforce isolation, the process abstraction provides the illusion to a program that it has its own private machine. 

A process provides a program with what appears to be a private memory system and its own CPU to execute.

**Xv6 use page tables to give each process its own address space which translates a virtual address to a physical address.**

Xv6 maintains a separate page table for each process that defines that process's address space.

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/page-table.png" alt="system call" style="max-width: 40%">

The xv6 kernel maintains many pieces of state for each process.

A process's most important pieces of kernel state are its page table, kernel stack and run state.

**Each process has a thread of execution (thread) that executes the process's instructions.**

A thread can be suspended and later resumed, to switch transparently between processes, 

the kernel suspends the currently running thread and resumes another process's thread.