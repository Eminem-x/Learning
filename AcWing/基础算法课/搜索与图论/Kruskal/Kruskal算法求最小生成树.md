### Kruskal算法求最小生成树

> 给定一个 n 个点 m 条边的无向图，图中可能存在重边和自环，边权可能为负数。
>
> 求最小生成树的树边权重之和，如果最小生成树不存在则输出 `impossible`。
>
> 给定一张边带权的无向图 G=(V,E)，其中 V 表示图中点的集合，E 表示图中边的集合，n=|V|，m=|E|。
>
> 由 V 中的全部 n 个顶点和 E 中 n−1 条边构成的无向连通子图被称为 G 的一棵生成树，
>
> 其中边的权值之和最小的生成树被称为无向图 G 的最小生成树。

1. 将所有边按权重从小到大排序
2. 枚举每条边：`x` 、`y`、`w`，如果两个点不连通，将这条边加入集合中
3. 即排序 + 并查集即可，需要注意如果数组实现特殊处理额外开辟的一个点权重值

代码如下：

```java
import java.util.*;

class Main {
    public static void main(String[] args) {
        new Main().solution();
    }
    
    int n, m;
    int N = 200010;
    int[] p = new int[N];
    int[][] edge = new int[N][3];

    int find(int x) {
        if (p[x] != x) {
            p[x] = find(p[x]);
        }
        return p[x];
    }
    
    void solution() {
        Scanner in = new Scanner(System.in);
        n = in.nextInt();
        m = in.nextInt();
        edge = new int[m + 1][3];
        // 特殊处理 0 点
        edge[0][2] = -(1 << 30 - 1);
        for (int i = 1; i <= n; i++) {
            p[i] = i;
        }
        for (int i = 1; i <= m; i++) {
            edge[i][0] = in.nextInt();
            edge[i][1] = in.nextInt();
            edge[i][2] = in.nextInt();
        }
        Arrays.sort(edge, (o1, o2) -> o1[2] - o2[2]);
        
        int res = 0;
        int cnt = 0;
        for (int i = 1; i <= m; i++) {
            int x = edge[i][0];
            int y = edge[i][1];
            int w = edge[i][2];
            x = find(x);
            y = find(y);
            if (x != y) {
                p[x] = y;
                cnt++;
                res += w;
            }
        }
        
        if (cnt < n - 1) {
            System.out.println("impossible");
        } else {
            System.out.println(res);
        }
    }
}
```

