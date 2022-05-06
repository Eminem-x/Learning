### Prim算法求最小生成树

> 给定一个 n 个点 m 条边的无向图，图中可能存在重边和自环，边权可能为负数。
>
> 求最小生成树的树边权重之和，如果最小生成树不存在则输出 `impossible`。
>
> 给定一张边带权的无向图 G=(V,E)，其中 V 表示图中点的集合，E 表示图中边的集合，n=|V|，m=|E|。
>
> 由 V 中的全部 n 个顶点和 E 中 n−1 条边构成的无向连通子图被称为 G 的一棵生成树，
>
> 其中边的权值之和最小的生成树被称为无向图 G 的最小生成树。

1. 采用邻接矩阵实现，初始化矩阵中所有点负无穷，而后根据输入更新每条边的最小值
2. 将 `dist` 数组初始化为负无穷
3. `n` 次循环遍历，每次分为以下三步：
   * 找到距离集合最近的点，并且此点不在集合内
   * 将这个点加入到集合内
   * 用这个点更新其他点到集合的距离
4. 过程中如果出现不连通的情况，说明无最小生成树，否则返回结果即可。

<strong>需要注意避免自负环带来的影响，因此先将点加入集合内，再利用其去更新其他点。</strong>

同 `Dijsktra` 也可以进行优化，即堆优化。

代码如下：

```java
import java.util.*;

class Main {
    public static void main(String[] args) {
        new Main().solution();
    }
    
    int N = 510;
    int n, m;
    int [][] g = new int[N][N];
    int[] dist = new int[N];
    boolean[] st = new boolean[N];
    int INF = 1 << 30 - 1;
    
    int prim() {
        int res = 0;
        Arrays.fill(dist, INF);
        
        for (int i = 0; i < n; i++) {
            int t = -1;
            // 未在集合中的点，距离集合最近的
            for (int j = 1; j <= n; j++) {
                if (!st[j] && (t == -1 || dist[t] > dist[j])) {
                    t = j;
                }
            }
            
            // 如果当前点不连通
            if (i != 0 && dist[t] == INF) {
                return INF;
            }
            
            // 更新值
            if (i != 0) {
                res += dist[t];
            }
            // 避免出现负环
            st[t] = true;
            
            // 用 t 去更新未在集合中的点
            for (int j = 1; j <= n; j++) {
                dist[j] = Math.min(dist[j], g[t][j]);
            }
        }
        return res;
    }
    
    void solution() {
        Scanner in = new Scanner(System.in);
        n = in.nextInt();
        m = in.nextInt();
        for (int i = 0; i < N; i++) {
            Arrays.fill(g[i], INF);
        }
        
        for (int j = 0; j < m; j++) {
            int x = in.nextInt();
            int y = in.nextInt();
            int w = in.nextInt();
            g[x][y] = g[y][x] = Math.min(g[x][y], w); 
        }
        
        int res = prim();
        if (res == INF) {
            System.out.println("impossible");
        } else {
            System.out.println(res);
        }
    }
}
```

