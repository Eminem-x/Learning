func minimumAbsDifference(arr []int) [][]int {
    sort.Ints(arr)
    ans := make([][]int, 0)
    min := math.MaxInt32
    for i := 0; i < len(arr) - 1; i++ {
        if arr[i + 1] - arr[i] < min {
            min = arr[i + 1] - arr[i]
            ans = make([][]int, 0)
            ans = append(ans, []int{arr[i], arr[i + 1]})
        } else if arr[i + 1] - arr[i] == min {
            ans = append(ans, []int{arr[i], arr[i + 1]})
        }
    }
    return ans
}
