func oddCells(m int, n int, indices [][]int) int {
    arr := make([][]int, m)
    for i := 0; i < m; i++ {
        arr[i] = make([]int, n)
    }

    for i := 0; i < len(indices); i++ {
        row := indices[i][0]
        col := indices[i][1]
        for j := 0; j < n; j++ {
            arr[row][j]++
        }
        for j := 0; j < m; j++ {
            arr[j][col]++
        }
    }

    var res int
    for i := 0; i < m; i++ {
        for j := 0; j < n; j++ {
            if arr[i][j] % 2 != 0 {
                res++
            }
        }
    }
    
    return res
}
