### spfa求最短路

> 给定一个 n 个点 m 条边的有向图，图中可能存在重边和自环， **边权可能为负数**。
>
> 请你求出 1 号点到 n 号点的最短距离，如果无法从 1 号点走到 n 号点，则输出 `impossible`。
>
> 数据保证不存在负权回路。

`Dijkstra` 算法是无法求边权为负数的情况，并且优化后的时间复杂度为O(mlogn)。

<strong>另外如果存在负权回路，是无法用该方法的，但是可以用其去判断是否存在负权回路。</strong>

`spfa` 算法是基于 `Buffman-Ford` 算法的改进，在 `BF` 算法中，其中更新距离操作中，有许多边其实是不用更新的。

<strong>因为只有已经发生更新的点，才能对其后的点距离造成影响，</strong>故可以用队列存储发生更新的点，让其去更新其后的点，

这点很像 `Dijkstra` 算法，具体实现过程中，记录已经入队的点，避免重复入队，增加时间复杂度。

至于 `spfa` 的时间复杂度，如果不存在卡数据的情况是很快的，最坏为O(mn)，即与朴素 `Buffman-Ford `算法一样。

代码如下：

```java
import java.util.*;

class Main {
    public static void main(String[] args) {
        new Main().solution();
    }
    
    int n, m, ans;
    int idx = 0;
    int N = 100010;
    int INF = 1 << 30 - 1;
    int[] h = new int[N];
    int[] e = new int[N];
    int[] ne = new int[N];
    int[] w = new int[N];
    int[] dist = new int[N];
    boolean[] st = new boolean[N];
    
    void add(int x, int y, int w) {
        e[idx] = y; this.w[idx] = w; ne[idx] = h[x]; h[x] = idx++;
    }
        
    
    void solution() {
        Scanner in = new Scanner(System.in);
        n = in.nextInt();
        m = in.nextInt();
        Arrays.fill(h, -1);
        while (m-- != 0) {
            add(in.nextInt(), in.nextInt(), in.nextInt());
        }
        spfa();
        if (dist[n] == INF) {
            System.out.println("impossible");
        } else {
            System.out.println(dist[n]);
        }
    }
    
    void spfa() {
        Queue<Integer> q = new LinkedList<>();
        Arrays.fill(dist, INF);
        dist[1] = 0;
        q.offer(1);
        st[1] = true;
        
        while (!q.isEmpty()) {
            int t = q.poll();
            st[t] = false;
            
            for (int i = h[t]; i != -1; i = ne[i]) {
                int j = e[i];
                if (dist[j] > dist[t] + w[i]) {
                    dist[j] = dist[t] + w[i];
                    if (!st[j]) {
                        q.offer(j);
                        st[j] = true;
                    }
                }
            }
        }
    }
}
```

