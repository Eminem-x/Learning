func strongPasswordCheckerII(password string) bool {
	if len(password) < 8 {
		return false
	}
	str := "!@#$%^&*()-+"
	var lower, upper, digit, special int
	var c rune
	for _, v := range password {
		if v >= '0' && v <= '9' {
			digit++
		}
		if v >= 'a' && v <= 'z' {
			lower++
		}
		if v >= 'A' && v <= 'Z' {
			upper++
		}
		for _, t := range str {
			if t == v {
				special++
			}
		}
		if c == v {
			return false
		} else {
			c = v
		}
	}
	return lower > 0 && upper > 0 && digit > 0 && special > 0
}

func strongPasswordCheckerII(password string) bool {
	n := len(password)
	if n < 8 {
		return false
	}

	var hasLower, hasUpper, hasDigit, hasSpecial bool
	for i, ch := range password {
		if i != n-1 && password[i] == password[i+1] {
			return false
		}
		if unicode.IsLower(ch) {
			hasLower = true
		} else if unicode.IsUpper(ch) {
			hasUpper = true
		} else if unicode.IsDigit(ch) {
			hasDigit = true
		} else if strings.ContainsRune("!@#$%^&*()-+", ch) {
			hasSpecial = true
		}
	}

	return hasLower && hasUpper && hasDigit && hasSpecial
}
