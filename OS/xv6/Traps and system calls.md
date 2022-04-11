### Traps and system calls

----

### Traps

There are three kinds of event which cause the CPU to set aside ordinary execution of instructions 

and force a transfer of control to special code that handles the event.

* <strong>system call</strong>, when a user program executes the `ecall` instruction to ask the kernel to do something.
* <strong>exception</strong>, an instruction (user or kernel) does something illegal, such as divide by zero or an invalid virtual address.
* <strong>device interrupt</strong>, when  a device signals that it needs attention.

<br>

<strong>Trap</strong> as a generic term for these situations and we want traps to be transparent.

Whatever code was executing at the time of the trap will later need to resume,

and shouldn't need to be aware that anything special happened.

<br>

The usual sequence is that a trap forces a transfer of control into the kernel;

the kernel saves registers and other state so that execution can be resumed;

the kernel executes appropriate handler code and restores the saved state and return from the trap;

and the original code resumes where it left off.

<br>

<strong>Xv6 trap handling proceeds in four stages:</strong>

* hardware actions taken by the RISC-V CPU
* an assembly 『vector』that prepares the way for kernel C code
* a C trap handler that decides what to do with the trap
* the system call or device-driver service routine

While commonality among the three trap types suggests that a kernel could handle all traps with a single code path,

it turns out to be convenient to have separate assembly vectors and C trap handlers for three distinct cases:

<strong>trap from user space, trap form kernel space, and timer interrupts.</strong>

----

### RISC-V trap machinery

Each RISC-V CPU has a set of control registers that the kernel writes to tell the CPU how to handle traps,

and that the kernel can read to find out about a trap that has occurred.

<br>

`riscv.h` contains definitions that xv6 uses. <strong>Here's an outline of the most important registers:</strong>

* `stvec` : The kernel writes the address of its trap handler here; the RISC-V jumps here to handler a trap

* `sepc`: When a trap occurs, RISC-V save the program counter here (since the `pc` is then overwritten with `stvec`). 

  The `sret` (return from trap) instruction copies `sepc` to the `pc`.

  The kernel can write to `sepc` to control where `sret` goes.

* `scause`: The RISC-V puts a number here that describes the reason for the trap.

* `sscratch`: The kernel places a value here that comes in handy at the very start of a trap handler.

* `sstatus`: The `SIE` bit controls whether device interrupts are enabled.

  If the kernel clears `SIE`, the RISC-V will defer device interrupts until the kernel sets `SIE`.

  The `SPP` bit indicates whether a trap came from user mode or supervisor mode,

  and controls to what mode `sret` return.

`stvec`：Supervisor Trap-Vector Base Address, low two bits are mode.

`sepc`：Machine exception program counter, holds the instruction address to which a return from exception will go.

`scause`：Supervisor Trap Cause

`sscratch`：Supervisor Scratch register, for early trap handler in trampoline.S

`sstatus`：Supervisor Status，`SIE`：Supervisor Interrupt Enable，`SPP`：Previous mode, 1=Supervisor, 0=User

<br>

The above registers relate to traps handled in supervisor mode, and they cannot be read or written in user mode.

There is an equivalent set of control registers for traps handled in machine mode;

xv6 uses them only for the special case of timer interrupts.

<br>

When it needs to force a trap, the RISC-V hardware does the following for all trap types <strong>(other than timer interrupts)</strong>:

1. If the trap is a device interrupt, and the `sstatus` SIE bit is clear, don't do any of the following.
2. Disable interrupts by clearing SIE.
3. Copy the `pc` to `sepc`.
4. Save the current mode (user or supervisor) in the SPP bit in `sstatus`.
5. Set `scause` to reflect the trap's cause.
6. Set the mode to supervisor.
7. Copy `stvec` to the `pc`.
8. Start executing at the new `pc`.

需要注意 CPU 不在内核中切换内核页表，也不切换栈，除了 `pc` 以外也不存储任何寄存器，

内核软件必须完成这些任务，减轻 CPU 的工作量并且也给软件提供了灵活性，能够提高性能。

---

### Traps from user space



---

### Code: Calling system calls

第二章（Operating system organization）以在 `initcode.S` 中执行 `exec` 系统调用结束，

结合接下来的过程，来学习用户调用如何在内核中实现 `exec` 系统调用的。

<br>

The user code places the arguments for `exec` in registers `a0` and `a1`, and puts the system call number in `a7`. 

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/initcode.png" alt="system call" style="max-width: 60%">

System call numbers match the entries in the `syscalls` array, a table of function pointers.

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/syscalls.png" alt="system call" style="max-width: 60%">

The `ecall` instruction traps into the kernel and executes `uservec`, `usertrap`, and then `syscall`, as we saw above.

<br>

`syscall` retrieves the system call number from the saved `a7` in the trapframe and uses it to index into `syscalls`.

When the system call implementation function returns, `syscall` records its return value in `p->trapframe->a0`.

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/syscall.png" alt="system call" style="max-width: 60%">

这将导致原始用户空间对 `exec` 的调用返回该值，因为 RISC-V 的 C 调用约定将返回值放在 `a0` 中，

系统调用通常返回负数表示错误，返回零或正数表示成功，如果系统调用号无效，`syscall` 打印错误并返回 -1。

---

### Code: System call arguments

在内核中的系统调用实现需要找到从用户代码传递的参数。

<strong>因为用户代码调用系统调用的封装函数（wrapper functions），所以参数最初被放置在RISC-V C调用所约定的地方：寄存器。</strong>

The kernel trap code saves user registers to the current process's trap frame, where kernel code can find them.

The functions `argint`, `argaddr`, `argfd` retrieve the n'th system call argument from the trap frame as an integer, pointer or a fd.

They all call `argraw` to retrieve the appropriate saved user register.<strong>上述四个函数均在 `kernel/syscall.h` 中实现。</strong>

<br>

有些系统调用传递指针作为参数，内核必须使用这些指针来读取或写入用户内存。

例如：`exec `系统调用传递给内核一个指向用户空间中字符串参数的指针数组。这些指针带来了两个挑战。

首先，用户程序可能有缺陷或恶意，可能会传递给内核一个无效的指针，或者一个旨在欺骗内核访问内核内存而不是用户内存的指针。

其次，xv6内核页表映射与用户页表映射不同，因此内核不能使用普通指令从用户提供的地址加载或存储。

<br>

内核实现了安全地将数据传输到用户提供的地址和从用户提供的地址传输数据的功能。`fetchstr `是一个例子（***kernel/syscall.c***:25）。

文件系统调用，如 `exec`，使用 `fetchstr ` 从用户空间检索字符串文件名参数。`fetchstr`调用 `copyinstr` 来完成这项困难的工作。

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/fetchstr.png" alt="system call" style="max-width: 60%">

`copyinstr`（***kernel/vm.c***:406）从用户页表页表中的虚拟地址 `srcva` 复制 `max` 字节到 `dst`。

它使用 `walkaddr`（它又调用 `walk` ）在软件中遍历页表，以确定 `srcva` 的物理地址 `pa0` 。

由于内核将所有物理RAM地址映射到同一个内核虚拟地址，`copyinstr `可以直接将字符串字节从 `pa0` 复制到 `dst` 。

`walkaddr`（***kernel/vm.c***:95）检查用户提供的虚拟地址是否为进程用户地址空间的一部分，

因此程序不能欺骗内核读取其他内存。一个类似的函数 `copyout`，将数据从内核复制到用户提供的地址。

---

### Traps from kernel space



---

### Page-fault exceptions