### Nim游戏

>给定 n 堆石子，两位玩家轮流操作，每次操作可以从任意一堆石子中拿走任意数量的石子（可以拿完，但不能不拿），
>
>最后无法进行操作的人视为失败。问如果两人都采用最优策略，先手是否必胜。

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/AcWing/pic/Part1/Nim游戏.png" alt="system call" style="max-width: 65%">

````go
package main

import "fmt"
    
func main() {
    var n int
    res := 0
    fmt.Scan(&n)
    for i := 0; i < n; i++ {
        t := 0
        fmt.Scan(&t)
        res ^= t
    }
    if res != 0 {
        fmt.Println("Yes")
    } else {
        fmt.Println("No")
    }
}
````

