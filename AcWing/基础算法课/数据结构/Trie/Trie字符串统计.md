### Trie字符串统计

有关 `Trie` 部分的内容，可以参见博客：https://eminem-x.github.io/2022/03/24/%E5%AD%97%E5%85%B8%E6%A0%91/

> 维护一个字符串集合，支持两种操作：
>
> 1. `I x` 向集合中插入一个字符串 x；
> 2. `Q x` 询问一个字符串在集合中出现了多少次。
>
> 共有 N 个操作，输入的字符串总长度不超过 10<sup>5</sup>，字符串仅包含小写英文字母。

如果是在平常解题中，可以直接使用 `Map` 集合来解决，但是为了学习 `Trie`，所以实现字典树。

字典树（前缀树）常见的有两种实现方式：结构体和数组，二者的优劣在前面相关的数据结构中已经陈述了，

无非就是空间换时间和时间换空间的思想，当然具体实现会有差异，不过都可以解题，本篇采用数组实现。

代码如下：

```java
import java.util.*;
import java.io.*;

class Main {
    static int N = 100010;
    static int[][] son = new int[N][26];
    static int[] cnt = new int[N];
    static int idx = 0;
    
    public static void insert(String s) {
        int p = 0;
        for (int i = 0; i < s.length(); i++) {
            int t = s.charAt(i) - 'a';
            if (son[p][t] == 0) {
                son[p][t] = ++idx;
            }
            p = son[p][t];
        }
        cnt[p]++;
    }
    
    public static int query(String s) {
        int p = 0;
        for (int i = 0; i < s.length(); i++) {
            int t = s.charAt(i) - 'a';
            if (son[p][t] == 0) {
                return 0;
            }
            p = son[p][t];
        }
        return cnt[p];
    }
    
    public static void main(String[] args) throws IOException {
        Scanner in = new Scanner(System.in);
        BufferedWriter bw = new BufferedWriter(new OutputStreamWriter(System.out));
        
        while (in.hasNext()) {
            int n = in.nextInt();
            for (int i = 0; i < n; i++) {
                String op = in.next();
                String s = in.next();
                if (op.equals("Q")) {
                    int x = query(s);
                    bw.write(x + "");
                    bw.write("\n");
                }
                if (op.equals("I")) {
                    insert(s);
                }
            }
            bw.flush();
            bw.close();
        }
    }
}
```

有时候不得不吐槽，`Java` 的输入输出，以及全局变量的声明是多么的繁琐，不过也体会了 `N = 100010` 的便捷。