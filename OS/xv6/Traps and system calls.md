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



---

### Code: System call arguments



---

### Traps from kernel space



---

### Page-fault exceptions