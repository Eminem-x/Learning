func lenLongestFibSubseq(arr []int) int {
    var ans int
    set := make(map[int]bool)
    dp := make([][]int, len(arr))
    for i := 0; i < len(arr); i++ {
        set[arr[i]] = true
        dp[i] = make([]int, len(arr))
    }
    for i := 0; i < len(dp); i++ {
        for j := i + 1; j < len(dp); j++ {
            a, b, k := arr[i], arr[j], 2
            for ; set[a + b]; k++ {
                a, b = b, a + b
            }
            dp[i][j] = k
            ans = getMax(ans, dp[i][j])
        }
    }
    if ans == 2 {
        return 0
    }
    return ans
}

func getMax(a, b int) int {
    if a > b {
        return a
    }
    return b
}
