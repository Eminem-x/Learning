import "strings"

func evaluate(s string, knowledge [][]string) string {
	// 不能用 replace 函数会超时

	dict := map[string]string{}
	for _, kd := range knowledge {
		dict[kd[0]] = kd[1]
	}
	ans := &strings.Builder{}
	start := -1
	for i, c := range s {
		if c == '(' {
			start = i
		} else if c == ')' {
			if t, ok := dict[s[start+1:i]]; ok {
				ans.WriteString(t)
			} else {
				ans.WriteByte('?')
			}
			start = -1
		} else if start < 0 {
			ans.WriteRune(c)
		}
	}

	return ans.String()

	// 尝试正则表达式
	// reg, _ := regexp.Compile(`[\(（].*[\)）]`)
	// t := reg.FindStringSubmatch(s)
	// s = reg.ReplaceAllLiteralString(s, "?")
	// fmt.Println(s)
	// fmt.Println(reg.FindString(s))
}
