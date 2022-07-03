func nextGreaterElement(n int) int {
    // 字符串不可修改, 所以转为 []byte
    s := []byte(strconv.Itoa(n))

    // 后序寻找较小者
    i := len(s) - 2
    for i >= 0 && s[i] >= s[i + 1] {
        i--
    }
    // 找不到
    if i < 0 {
        return -1
    }

    // 后序寻找较大者
    j := len(s) - 1
    for j > i && s[i] >= s[j] {
        j--
    }
    
    // 交换较大较小
    s[i], s[j] = s[j], s[i]

    // 交换后序
    for i, j = i + 1, len(s) -1; i < j; i, j = i + 1, j - 1 {
        s[i], s[j] = s[j], s[i]
    }
    
    // 判断是否满足
    ans, _ := strconv.Atoi(string(s))
    if ans > math.MaxInt32 || ans == n {
        return -1
    }
    return ans
}
