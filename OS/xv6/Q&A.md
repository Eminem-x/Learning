### Question And Solve

### 1. 资源链接

* 课程 Schedule：https://pdos.csail.mit.edu/6.828/2020/schedule.html
* 课程视频回放：<a href="https://www.bilibili.com/video/BV19k4y1C7kA?spm_id_from=333.1007.top_right_bar_window_default_collection.content.click">MIT 6.S081</a>

这是最主要的两个资源链接，其余资源可在 Schedule 中找到

----

### 2. 配置环境

WSL（Windows Subsystem for Linux）、 Ubuntn，

可以在 Microsoft Store 中下载 Ubuntu on Windows

<img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/ubuntu.png" alt="system call" style="max-width: 100%;">

按照指示输入指令，如果出现以下错误

```html
Package gdb-multiarch is not available, but is referred to by another package. 
This may mean that the package is missing, has been obsoleted, 
or is only available from another source
Package 'gdb-multiarch' has no installation candidate
```

解决方法：

```html
# apt-get update
# apt-get upgrade
# apt-get install <packagename>
```

主要更新软件包和软件源

详细教程可以看博客：https://gwzlchn.github.io/202106/6-s081-lab0/

随后再返回官方文档去理解：https://pdos.csail.mit.edu/6.828/2020/tools.html

二者在配置环境有些出入，不过经过尝试，不会造成影响。

----

### 3. 如何按照要求增加功能

我在学习的过程中，误以为可以使用编译器去正常的运行代码，

不过发现不能在编译器中编译运行，可能会出现如下所示错误：

```c
PS C:\Users\DELL\xv6-labs-2020> cd "c:\Users\DELL\xv6-labs-2020\user\" ; if ($?) { gcc copy.c -o copy } ; if ($?) { .\copy }
copy.c:4:26: fatal error: kernel/types.h: No such file or directory
 #include "kernel/types.h"
```

后来经过请教，增加功能的方式，即按照实验指示即可：

* 编写一个 C 语言文件放入 user/ 路径下
* 然后在 Makefile 的 UPROGS 中配置
* 重新执行 make clean 和 make qemu 进行编译即可

其中 make clean 的功能大概是删除已经经过编译产生的文件，

make 执行后的结果是 编译 kernel 中的文件，

make qemu 即编译 user 中的文件以及整体的环境，

以上的说法可能不对，个人理解。

-----

### 4. 如何从命令行输入参数

其基本含义是，argc 代表向 main 函数中传入参数的数量，而 argv 代表传入的字符串构成的数组，

其默认 argv[0] 代表程序的名称，其它的 argv[1] 和 argv[2]… 根据传入的字符串的数量，依次向后排列。

所以没有传入参数时，argc = 1，而传入之后，argc = 传入参数的数量 + 1。

-----

### 5. 不能执行评分 Grade 功能

执行此功能需要退出模拟环境，来到目录下，按照指示执行即可，

在按照指示执行时，会报出下列错误：

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/grade.png" alt="system call" style="max-width: 80%;">

解决方法：https://askubuntu.com/questions/942930/usr-bin-env-python-no-such-file-or-directory

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/grade-slove.png" alt="system call" style="max-width: 80%;">

运行结果：

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/OS/pic/grade-success.png" alt="system call" style="max-width: 100%;">

---

### 6. 实验部分

1. 一定要将书上的部分理解透彻，尤其是代码流程的部分，尽管会觉得枯燥或者厌倦，但是一定要耐着性子读下去
2. 如果对于某些细节无法处理好，可以适当参考别人完成的代码，先看思路，如果还是写不出来，再看代码
3. 不在于快，在于精，重点其实还是在于整个操作系统的概念，代码部分是辅助，坚持下去。
