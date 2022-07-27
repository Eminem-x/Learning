func fractionAddition(expression string) string {
	numerator, denominator := make([]int, 0), make([]int, 0)

	// 特殊处理首位
	if expression[0] != '-' && expression[0] != '+' {
		expression = "+" + expression
	}

	for i := 0; i < len(expression); i += 4 {
		// 符号位
		flag := 1
		if expression[i] == '-' {
			flag = -1
		}
		// 1 - 10
		cnt, c := 0, expression[i+2]
		if c != '/' && i+2 < len(expression) {
			cnt++
			t, _ := strconv.Atoi(expression[i+1 : i+3])
			numerator = append(numerator, flag*t)
		} else {
			numerator = append(numerator, flag*int(expression[i+1]-'0'))
		}
		if cnt+i+4 < len(expression) {
			c = expression[cnt+i+4]
		}
		if cnt+i+4 < len(expression) && c != '+' && c != '-' {
			t, _ := strconv.Atoi(expression[cnt+i+3 : cnt+i+5])
			denominator = append(denominator, t)
			cnt++
		} else {
			denominator = append(denominator, int(expression[cnt+i+3]-'0'))
		}
		i += cnt
	}

	// 获得最简分子分母
	m, n := 0, 1
	for i := 0; i < len(denominator); i++ {
		if n%denominator[i] != 0 {
			n *= denominator[i]
		}
	}
	for i := 0; i < len(numerator); i++ {
		numerator[i] *= n / denominator[i]
		m += numerator[i]
	}

	// 得到结果
	flag := 1
	if m < 0 {
		flag = -1
		m = -m
	}

	t := getGcd(m, n)
	m, n = flag*m/t, n/t

	// 转为字符串
	return strconv.Itoa(m) + "/" + strconv.Itoa(n)
}

// 辗转相除法
func getGcd(x, y int) int {
	if y != 0 {
		return getGcd(y, x%y)
	}
	return x
}
