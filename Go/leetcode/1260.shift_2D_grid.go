func shiftGrid(grid [][]int, k int) [][]int {
    row, col := len(grid), len(grid[0])

    for ; k > 0; k-- {

        t := make([][]int, row)
        for i := 0; i < row; i++ {
            t[i] = make([]int, col)
        }

        for i := 0; i < row; i++ {
            for j := 0; j < col; j++ {
                x, y := i, j + 1
                if x == row - 1 && y == col {
                    x, y = 0, 0
                } else if y == col {
                    x, y = i + 1, 0
                } else {
                    x, y = i, j + 1
                }
                t[x][y] = grid[i][j]
            }
        }

        grid = t
    }

    return grid
}
