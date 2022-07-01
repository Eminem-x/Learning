func diffWaysToCompute(expression string) []int {
    if isDigit(expression) {
        temp, _ := strconv.Atoi(expression)
        return []int{temp}
    }

    var res []int
    for idx, c := range expression {
        if c == '+' || c == '*' || c == '-' {
            left := diffWaysToCompute(expression[:idx])
            right := diffWaysToCompute(expression[idx + 1:])

            for _, leftNum := range left {
                for _, rightNum := range right {
                    var t int
                    if c == '+' {
                        t = leftNum + rightNum                        
                    } else if c == '-' {
                        t = leftNum - rightNum
                    } else if c == '*' {
                        t = leftNum * rightNum
                    }
                    res = append(res, t)
                }
            }  
        }
    }
    return res
}

func isDigit(input string) bool {
    _, err := strconv.Atoi(input)
    return err == nil
}
