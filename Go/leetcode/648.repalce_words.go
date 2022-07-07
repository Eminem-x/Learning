func replaceWords(dictionary []string, sentence string) string {
    set := make(map[string]bool)
    for _, v := range dictionary {
        set[v] = true
    }

    var ans string

    strs := strings.Split(sentence, " ")

    for j := range strs {
        var t string
        for i := 0; i < len(strs[j]); i++ {
            t += string(strs[j][i])
            if set[t] {
                ans += t
                break;
            } else if i == len(strs[j]) - 1  {
                ans += strs[j]
            }
        }
        if j != len(strs) - 1 {
            ans += " "
        }
    }
    return ans
}
