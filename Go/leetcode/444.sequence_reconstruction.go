func sequenceReconstruction(nums []int, sequences [][]int) bool {
    const N, M = 10010, 20020
    var idx int
    var h, d, q [N]int
    var e, ne [M]int

    // 初始化为 -1
    for i := range h {
        h[i] = -1
    }

    // 构建图
    for _, v := range sequences {
        for i := len(v) - 1; i > 0; i-- {
            x, y := v[i], v[i - 1]
            e[idx] = y; ne[idx] = h[x]; h[x] = idx; idx++
            d[y]++
        }
    }

    // 拓扑排序
    hh, tt, n := 0, -1, len(nums)
    for i := 1; i <= n; i++ {
        if d[i] == 0 {
            tt++; q[tt] = i
        }
    }

    for hh <= tt {
        // 如果入度为 0 的点不唯一
        if tt - hh > 0 {
            return false
        }
        t := q[hh]; hh++
        for i := h[t]; i != -1; i = ne[i] {
            j := e[i]; d[j]--
            if d[j] == 0 {
                tt++; q[tt] = j
            }
        }
    }

    // 判断是否为全序列
    return tt == (n - 1)
}
