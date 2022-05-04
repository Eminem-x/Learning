### spfa判断负环

> 给定一个 n 个点 m 条边的有向图，图中可能存在重边和自环， **边权可能为负数**。
>
> 请你判断图中是否存在负权回路。

`Buffman-Ford` 算法判断负环，只需要循环 `n + 1` 次，最后一次判断 `backup`  和 `dist` 数组的值是否相等即可。

```java
static void bellmanFord() {
    Arrays.fill(dist, INF);
    dist[1] = 0;
    
    for (int i = 0; i < n + 1; i++) {
        System.arraycopy(dist, 1, backup, 1, n);
        for (int j = 0; j < list.size(); j++) {
            Edge edge = list.get(j);
            dist[edge.y] = Math.min(dist[edge.y], backup[edge.x] + edge.w);
        }
    }
    
    for (int i = 1; i <= n; i++) {
        if (dist[i] != backup[i]) {
            ans = -1;
            break;
        }
    }
}
```

而一般采用的是 `spfa` 算法判断是否存在负环，判断方式非常巧妙，就是在原图上构建一个虚拟源点，其到所有点的距离都是 `0`，

然后利用这个点循环遍历更新其他点，即 `spfa` 算法，循环过程中记录每个点的边数，如果大于等于 `n` 说明存在负环。

````java
import java.util.*;

class Main {
    public static void main(String[] args) {
        new Main().solution();
    }
    
    int n, m, ans;
    int idx = 0;
    int N = 10010;
    int INF = 1 << 30 - 1;
    int[] h = new int[N];
    int[] e = new int[N];
    int[] ne = new int[N];
    int[] w = new int[N];
    int[] dist = new int[N];
    int[] cnt = new int[N];
    boolean[] st = new boolean[N];
    
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
            System.out.println("Yes");
        } else {
            System.out.println("No");
        }
    }
    
    void add(int x, int y, int w) {
        e[idx] = y; this.w[idx] = w; ne[idx] = h[x]; h[x] = idx++;
    }
    
    void spfa() {
        Queue<Integer> q = new LinkedList<>();
        for (int i = 1; i <= n; i++) {
            q.offer(i);
            st[i] = true; 
        }
        
        while (!q.isEmpty()) {
            int t = q.poll();
            st[t] = false;
            
            for (int i = h[t]; i != -1; i = ne[i]) {
                int j = e[i];
                if (dist[j] > dist[t] + w[i]) {
                    dist[j] = dist[t] + w[i];
                    cnt[j] = cnt[t] + 1;
                    if (cnt[j] > n - 1) {
                        dist[n] = INF;
                        return;
                    }
                    if (!st[j]) {
                        q.offer(j);
                        st[j] = true;
                    }
                }
            }
        }
    }
}
````

