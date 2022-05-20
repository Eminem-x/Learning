### 求组合数 II

>1 ≤ n ≤ 10000，1 ≤ b ≤ a ≤ 10<sup>5</sup>

数据规模增大，不能双重循环预处理，单次遍历求阶乘，需要用到逆元的知识，代码如下：

````java
import java.util.*;

class Main {
    public static void main(String[] args) {
        new Main().solution();
    }
    
    int N = 100010;
    int MOD = (int)1e9 + 7;
    int[] fact = new int[N];
    int[] infact = new int[N];
    Scanner in = new Scanner(System.in);
    
    long qmi(long a, long b, long p) {
        long res = 1;
        while (b != 0) {
            if ((b & 1) == 1) {
                res = res * a % p;   
            }
            b >>= 1;
            a = a * a % p;
        }
        return res;
    }
    
    void solution() {
        fact[0] = 1;
        infact[0] = 1;
        for (int i = 1; i < N; i++) {
            fact[i] = (int)((long)fact[i - 1] * i % MOD);
            infact[i] = (int)((long)infact[i - 1] * qmi(i, MOD - 2, MOD) % MOD);
        }
        
        int n = in.nextInt();
        while (n-- != 0) {
            int a = in.nextInt();
            int b = in.nextInt();
            System.out.println((long)fact[a] * infact[a - b] % MOD * infact[b] % MOD);
        }
    }
}
````

