func prefixCount(words []string, pref string) int {
	length := len(pref)
	var res int
	for _, word := range words {
		if len(word) >= length && word[:length] == pref {
			res++
		}
	}
	return res
}
