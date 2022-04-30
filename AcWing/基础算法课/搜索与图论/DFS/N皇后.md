### N皇后

经典问题，代码并不复杂，对于每行判断每个点是否满足而后递归即可，判断依据主要是行、列、正反对角线是否冲突。

值得讨论的是时间复杂度：最开始我觉得时间复杂度应该是 O(n<sup>n</sup>)，因为每个点遍历一次，然后递归继续遍历，

但是看了网上许多帖子都是说其时间复杂度为 O(n!) ，其实思考就不难得出原因，因为深度搜索过程中存在剪枝。

每个行在递归过程中，实际上每次只能有上一次选择 `-1` 的选择，因此是阶乘，具体推导就不深究了。

<br>

另外题外话，在 LeetCode 平台提交相同代码后，不通过，提交了 `issue`，得到的解答是不要使用 `static`，

评测机的原因吧，也希望以后主要，毕竟实现函数和实现全代码方式是不一样的。

<br>

不做特殊处理代码：

```java
import java.util.*;
import java.io.*;

class Main {
    static int n;
    static char[][] board;
    
    static void dfs(int x) {
        if (x == n) {
            for (int i = 0; i < n; i++) {
                for (int j = 0; j < n; j++) {
                    if (board[i][j] != 'Q') {
                        System.out.print(".");
                    } else {
                        System.out.print("Q");
                    }
                }
                System.out.println();
            }
            System.out.println();
            return;
        }
        
        // 遍历 x i 位置是否可取
        for (int i = 0; i < n; i++) {
            boolean flag = true;
            
            // 不需要判断当前行
            
            // 如果当前列已经存在皇后
            for (int j = 0; j < n && flag; j++) {
                if (board[j][i] == 'Q') {
                    flag = false;
                }
            }
            
            // 判断对角线
            int r = Math.max(x - i, 0);
            int c = Math.max(i - x, 0);
            for (int k = r, j = c; flag && k < n && j < n; k++, j++) {
                if (board[k][j] == 'Q') {
                    flag = false;
                }
            }
            
            c = Math.min(x + i, n - 1);
            for (int k = x, j = i; flag && k >= 0 && j <= c; k--, j++) {
                if (board[k][j] == 'Q') {
                    flag = false;
                }
            }
            
            if (flag) {
                board[x][i] = 'Q';
                dfs(x + 1);
                board[x][i] = '.';
            }
        }
    }
    
    public static void main(String[] args) {
        Scanner in = new Scanner(System.in);
        n = in.nextInt();
        board = new char[n][n];
        dfs(0);
    }
}
```

将对角线、行列存储优化后

```java
import java.util.*;
import java.io.*;

class Main {
    static int n;
    static int N = 10;
    static char[][] board = new char[N][N];
    static boolean[] row = new boolean[N];
    static boolean[] col = new boolean[N];
    static boolean[] dg = new boolean[N * 2];
    static boolean[] udg = new boolean[N * 2];
    
    static void dfs(int x) {
        if (x == n) {
            for (int i = 0; i < n; i++) {
                for (int j = 0; j < n; j++) {
                    if (board[i][j] != 'Q') {
                        System.out.print(".");
                    } else {
                        System.out.print("Q");
                    }
                }
                System.out.println();
            }
            System.out.println();
            return;
        }
        
        // 遍历 x i 位置是否可取
        for (int i = 0; i < n; i++) {
            if (!row[x] && !col[i] && !dg[x + i] && !udg[n - i + x]) {
                row[x] = true; col[i] = true;
                dg[x + i] = true; udg[n - i + x] = true;
                board[x][i] = 'Q';
                dfs(x + 1);
                board[x][i] = '.';
                row[x] = false; col[i] = false;
                dg[x + i] = false; udg[n - i + x] = false;
            }
        }
    }
    
    public static void main(String[] args) {
        Scanner in = new Scanner(System.in);
        n = in.nextInt();
        dfs(0);
    }
}
```