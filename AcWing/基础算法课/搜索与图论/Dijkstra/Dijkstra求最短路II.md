### Dijkstra求最短路II

增加了数据量，从稠密图变成了稀疏图，因此如果在用二维矩阵会超出内存限制，并且双重循环超出时间限制，

结合下图，分析朴素算法的时间复杂度，以及哪里可以进行优化：

<img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/AcWing/pic/Part1/dijkstra.png" alt="system call" style="max-width: 65%">

<br>

具体实现代码需要注意的地方：

1. 邻接表的 `add` 需要更新边的权重值，但是不需要考虑去重，因为优先队列可以存储；
2. 在更新其他结点值的过程中，变量下标的关系需要注意；
3. 同朴素写法，不能忘记更新距离为无穷，以及数组模拟链表的 `-1` 初始化；
4. 优先队列的 `lambda` 写法，对象类型需要一个排序依据，另外需要逻辑自洽，满足可逆排序。

代码如下：

```java
import java.util.*;

class Main {
    static int n, m;
    static int ans;
    static int idx = 0;
    static int N = 150010;
    static int[] h = new int[N];
    static int[] w = new int[N];
    static int[] e = new int[N];
    static int[] ne = new int[N];
    static int[] dist = new int[N];
    static boolean[] st = new boolean[N];
    static int INF = 1 << 30 - 1;
    
    public static void main(String[] args) {
        Scanner in = new Scanner(System.in);
        n = in.nextInt();
        m = in.nextInt();
        Arrays.fill(h, -1);
        while (m-- != 0) {
            int x = in.nextInt();
            int y = in.nextInt();
            add(x, y, in.nextInt());
        }
        dijkstra();
        System.out.println(ans);
    }
    
    static void add(int x, int y, int c) {
        e[idx] = y; w[idx] = c; ne[idx] = h[x]; h[x] = idx++; 
    }
    
    static void dijkstra() {
        Arrays.fill(dist, INF);
        dist[1] = 0;
        
        PriorityQueue<int[]> pq = new PriorityQueue<>((o1, o2) -> o1[1] - o2[1]);
        pq.offer(new int[] {1, 0});
        
        while (!pq.isEmpty()) {
            int v = pq.peek()[0];
            int dis = pq.peek()[1];
            pq.poll();
            
            if (st[v]) {
                continue;
            }
            st[v] = true;
            
            for (int i = h[v]; i != -1; i = ne[i]) {
                int j = e[i];
                if (dist[j] > dist[v] + w[i]) {
                    dist[j] = dist[v] + w[i];
                    pq.offer(new int[] {j, dist[j]});
                }
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

