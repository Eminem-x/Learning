### 求组合数 I

> 给定 n 组询问，每组询问给定两个整数 a，b，请你输出 C<sup>b</sup><sub>a</sub>mod(10<sup>9</sup> + 7) 的值。
>
> 1 ≤ n ≤ 10000，1 ≤ b ≤ a ≤ 2000

求组合数一般是看数据范围，有不同的解法，该题目双重循环不太好，需要用到组合数的递推式：

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/AcWing/pic/Part1/组合递推.png" alt="system call" style="max-width: 65%">

其实如何判断什么时候溢出，一直不是很熟练，感觉需要练习，对数据更加敏感。

```go
package main

import "fmt"

const N = 2010
const MOD = 1e9 + 7

func main() {
    var n int
    fmt.Scan(&n)
    
    dp := make([][N]int, N)
    
    for i := 0; i < N; i++ {
        for j := 0; j <= i; j++ {
            if j == 0 {
                dp[i][j] = 1
            } else {
                dp[i][j] = (dp[i - 1][j] + dp[i - 1][j - 1]) % MOD
            }
        }
    }
    
    for i := 0; i < n; i++ {
        var x, y int
        fmt.Scan(&x, &y)
        fmt.Println(dp[x][y])
    }
}
```

