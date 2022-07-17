func arrayNesting(nums []int) int {
    set := make(map[int]bool)
    hashMap := make(map[int]int)

    for i, v := range nums {
        hashMap[i] = v
    }

    var ans int

    for _, v := range nums {
        if set[v] {
            continue
        }
        set[v] = true

        var cnt int
        t := make(map[int]bool)
        for j, ok := hashMap[v]; ok && !t[j]; j = hashMap[j] {
            t[j] = true
            set[j] = true
            cnt++
        }

        if ans < cnt {
            ans = cnt
        }
    }

    return ans
}
