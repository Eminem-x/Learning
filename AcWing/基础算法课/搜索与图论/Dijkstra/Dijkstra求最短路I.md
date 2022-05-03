### Dijkstra求最短路I

> 给定一个 n 个点 m 条边的有向图，图中可能存在重边和自环，所有边权均为正值。
>
> 请你求出 1 号点到 n 号点的最短距离，如果无法从 1 号点走到 n 号点，则输出 −1。

在此之前先重述一下邻接表和邻接矩阵的区别：

* 邻接表：用于稀疏图——点多路径少，模拟链表的方式实现
* 邻接矩阵：用于稠密图——点多路径多，二维矩阵实现

接下来阐述的 `Dijkstra` 朴素算法的基本步骤：

1. 初始化点与点之间的权重值以及方向，不存在的正无穷表示；
2. 初始化 `dist` 该数组表示离 `1` 的距离，`1` 已知为 `0`，其余均为正无穷；
3. 随后进行迭代循环，循环 `n` 次，但实际 `n - 1` 次即可，因为终点不需要再判断路径长度：
   1. 在未遍历过的点中寻找 `dist` 最小的点 `t`；
   2. 用 `t` 去更新其他结点的最短路径，并将其标为已遍历。
4. 根据 `dist[n]` 的值得到是否可达，以及最短路径。

<br>

具体代码实现遇到的问题：

1. 无穷值的表示采用 2<sup>30</sup> - 1，避免两数相加溢出；
2. 切记为两个数组都赋无穷大值，并且更新时候需要考虑重边的影响；
3. 朴素写法的时间复杂度是 O(n<sup>2</sup>)，数据大的时候会 `TLE`。

代码如下：

```java
import java.util.*;

class Main {
    static int n, m;
    static int ans;
    static int N = 510;
    static int[][] grid = new int[N][N];
    static int[] dist = new int[N];
    static boolean[] st = new boolean[N];
    static int INF = 1 << 30 - 1;
    
    public static void main(String[] args) {
        Scanner in = new Scanner(System.in);
        n = in.nextInt();
        m = in.nextInt();
        for (int i = 0; i < n; i++) {
            Arrays.fill(grid[i], INF);
        }
        while (m-- != 0) {
            int x = in.nextInt();
            int y = in.nextInt();
            grid[x][y] = Math.min(in.nextInt(), grid[x][y]);
        }
        dijkstra();
        System.out.println(ans);
    }
    
    static void dijkstra() {
        Arrays.fill(dist, INF);
        dist[1] = 0;
        
        for (int i = 0; i < n; i++) {
            int t = -1;
            for (int j = 1; j <= n; j++) {
                if (!st[j] && (t == -1 || dist[j] < dist[t])) {
                    t = j;
                }
            }
            st[t] = true;
            for (int j = 1; j <= n; j++) {
                dist[j] = Math.min(dist[j], dist[t] + grid[t][j]);
            }
        }
        
        if (dist[n] == INF) {
            ans = -1;
        } else {
            ans = dist[n];
        }
    }
}
```

