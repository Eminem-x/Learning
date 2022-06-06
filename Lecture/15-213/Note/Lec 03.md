### Let 03

1. 前半段主要介绍补码的优势，给出了几个 demo，我在这里直接总结一下：

   * 溢出相当于对 2<sup>n</sup> 取模后的结果
   * 补码的优势在于不需要考虑符号
   * 正数转为负数二进制变换：取反加一

   这些知识，在计组和数逻课都有涉及，因此不是很难，不过有些地方理解方式不太相同

2. 移位的指令周期更短相比于乘法，尽管现代计算机乘法周期已经缩短了，但是除法依然很慢，

   在这里，有一处需要注意就是正数的移位如果直接照搬到负数，会产生不太正常的结果：

    <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/Lecture/15-213/pic/bias-example.png" alt="system call" style="max-width: 60%">

   解决方法就是：增加一个偏移量（add a bias）

3. 算数移位（arithmetic shift）：带有符号的数据，所以我们不能直接移动所有的位数

   逻辑移位（logic shift）：不考虑符号位，移位的结果只是数据所有的位数进行移位

4. 另外一个地方需要注意的就是，如何编写 secure code：

    <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/Lecture/15-213/pic/secure.png" alt="system call" style="max-width: 60%">

5. 关于虚拟内存，教授谈到现在的 64 位机，一般也是只用 47 位，并且关于 machine words 并无准确定义

   （利用 2<sup>10</sup> = 10<sup>3</sup> 近似计算得出 128 * 10<sup>12</sup> = 2<sup>47</sup>）

   另外 `gcc` 在编译的时候可以选择机器模式是 32 位还是 64 位，说明决定因素不仅仅在于硬件还与编译器有关，

   <strong>硬件本身并不一定定义字长大小，是硬件和编译器一起决定的</strong>

6. 对齐字节（aligned words），编译器通常非常难以保持对齐，

   谈及到字节的排序方式（Byte Ordering），目前分为大端序（<strong>Internet</strong>）和小端序，但是主流是小端序：
    <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/Lecture/15-213/pic/byte-ordering.png" alt="system call" style="max-width: 60%">

   大端序可读性比较好，小端序可读性较差，但是广为使用：

    <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/Lecture/15-213/pic/byte-order-example.png" alt="system call" style="max-width: 60%">