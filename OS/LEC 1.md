### LEC 1

----

### Overview

1. **6.S081 goals**
   * Understand operating system design and implementation
   * Hands-on experience extending a small O/S
   * Hands-on experience writing systems software
2. **The purpose of an O/S**
   * Abstract the hardware for convenience and portability
   * Multiplex the hardware among many applications
   * Isolate applications in order to contain bugs
   * Allow sharing among cooperating applications
   * Control sharing for security
   * Don't get in the way of high performance
   * Support a wide range of applications
3. **The kernel typically provide services**
   * process (a running program)
   * memory allocation
   * file contents
   * file names, directories
   * access control (security)
   * many others : users, network, time, terminals
4. **Why is O/S design + implementation hard and interesting**
   * unforgiving environment : quirky h/w, hard to debug
   * many design tensions:
     * efficient vs abstract/portable/general-purpose
     * powful vs simple interfaces
     * flexible vs secure
   * features interact
   * uses are varied : laptops, phones, cloud, virtual machines, embeded
   * evolving h/w

-----

### Introduction to UNIX system calls

1. Applications see the O/S via system call
2. xv6 has similar structure to UNIX systems but much simpler
3. Something else has been writted in the book, explaining the difficult points next

<img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/system%20call.png" alt="system call" style="max-width: 100%;">

----

### Process and memory

1. `Fork` returns in both the parent and the child, 

   In the parent, returns the child’s PID; in the child, returns zero.

2. `exit` causes the calling process to stop executing and to release resources,

   It takes an integer status argument, conventionally 0 to indicate success and 1 to indicate failure

3. `fork` and `exec` are not combined in a single call,
   and the shell exploits the separation in its implementation of I/O redirection.

4. `wait` copies the exit status of the child to the address passed to wait:

   * If one of the caller's children has exited or killed, return the PID of child.

   * If none of the caller's children has exited, waits for one to do so.
   * If the caller has no children, wait immediately returns -1.
   * If the parent doesn't care about the exit status, can wait((int *) 0) in this way.

----

### I/O and File Descriptors

**I recommend a blog to understand the fd: https://www.jianshu.com/p/a2df1d402b4d**

1. The `read` and `write` system calls read bytes from and 

   write bytes to open files named by file descriptors.

2. By convention, a process reads from file descriptor 0 (standard input),

   write output to file descriptor 1 (standard output),

   and writes error messages to file descriptor 2 (standard error).

3. The shell ensures that it always has three file descriptors open,

   which are by default file descriptors for the console.

4. The important thing to note is that code doesn’t know 

   whether it is reading from a file, console, or a pipe, or writting to.

   The photo may explain it:

   <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/fd.png" alt="system call" style="max-width: 100%;">

5. A process may obtain a file descriptor by opening a file, directory, or device,

   or by creating a pipe, or by duplicating an existing descriptor.

6. `close` releases a file descriptor, making it free for reuse in the future,

   a newly allocated file descriptor is always the lowest-numbered unused in the current process.

7. File descriptors and fork interact to make I/O redirection easy to implement.

8. `dup` duplicates an existing file descriptor, 

   returning a new one that refers to the same underlying I/O object.

9. Two file descriptors share an offset if they were derived from 

   the same original file descriptor by a sequence of fork and dup calls. 

   Otherwise file descriptors do not share offsets, 

   even if they resulted from open calls for the same file.

----

### Pipes







----

### File System
