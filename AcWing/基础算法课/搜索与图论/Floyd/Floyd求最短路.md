### Floyd求最短路

> 给定一个 n 个点 m 条边的有向图，图中可能存在重边和自环，边权可能为负数。
>
> 再给定 k 个询问，每个询问包含两个整数 x 和 y，表示查询从点 x 到点 y 的最短距离，如果路径不存在，则输出 `impossible`。
>
> 数据保证图中不存在负权回路。

`Floyd` 算法可以求出任意两点间的最短距离，核心思路就是，采用松弛，对在 `i` 和 `j` 之间的所有其他点进行一次松弛，

但是时间复杂度较高：O(n<sup>3</sup>)，一般用于稠密图和点的数量较少的情况，思想是贪心+动态规划。

其中状态方程：`d[k][i][j] = Math.min(d[k][i][j], d[k - 1][i][k] + d[k - 1][k][j]) `，具体实现时可以优化外层 `k`，

表示的含义是：经过 `1` 到 `k`  结点的 `i` 到 `j` 的距离为后两者的最小值。

代码如下：

````java
import java.util.*;

class Main {
    public static void main(String[] args) {
        new Main().solution();
    }
    
    int INF = (1 << 30) - 1;
    int n, m, q;
    int N = 210;
    int[][] d = new int[N][N];
    
    void solution() {
        Scanner in = new Scanner(System.in);
        n = in.nextInt();
        m = in.nextInt();
        q = in.nextInt();
        
        for (int i = 1; i <= n; i++) {
            for (int j = 1; j <= n; j++) {
                if (i == j) {
                    d[i][j] = 0;
                } else {
                    d[i][j] = INF;
                }
            }
        }
        
        while (m-- != 0) {
            int x = in.nextInt();
            int y = in.nextInt();
            int w = in.nextInt();
            d[x][y] = Math.min(d[x][y], w);
        }
        
        floyd();
        
        while (q-- != 0) {
            int x = in.nextInt();
            int y = in.nextInt();
            if (d[x][y] > INF / 2) {
                System.out.println("impossible");
            } else {
                System.out.println(d[x][y]);
            }
        }
    }
    
    void floyd() {
        for (int k = 1; k <= n; k++) {
            for (int i = 1; i <= n; i++) {
                for (int j = 1; j <= n; j++) {
                    d[i][j] = Math.min(d[i][j], d[i][k] + d[k][j]);
                }
            }
        }
    }
}
````

