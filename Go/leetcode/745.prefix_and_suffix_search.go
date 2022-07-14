type WordFilter struct {
    // 前缀数组存储
    preDict map[string][]int
    // 后缀哈希存储
    sufDict map[string]map[int]bool
}


func Constructor(words []string) WordFilter {
    var t WordFilter
    t.preDict = make(map[string][]int)
    t.sufDict = make(map[string]map[int]bool)

    for idx, v := range words {
        for i := 0; i < len(v); i++ {
            // 求出前缀
            pre := v[:i + 1]
            if t.preDict[pre] == nil {
                t.preDict[pre] = make([]int, 0)
            }
            t.preDict[pre] = append(t.preDict[pre], idx)

            // 求出后缀
            suf := v[len(v) - i - 1:]
            if t.sufDict[suf] == nil {
                t.sufDict[suf] = make(map[int]bool)
            }
            t.sufDict[suf][idx] = true
        }
    }

    return t
}


func (this *WordFilter) F(pref string, suff string) int {
    pre := this.preDict[pref]
    suf := this.sufDict[suff]
    for i := len(pre) - 1; i >= 0; i-- {
        if suf[pre[i]] {
            return pre[i]
        }
    }
    return -1
}
