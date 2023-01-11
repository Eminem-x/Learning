func digitCount(num string) bool {
	hashMap := make(map[rune]int)
	for _, v := range num {
		hashMap[v-'0']++
	}
	for i, v := range num {
		if hashMap[rune(i)] != int(v-'0') {
			return false
		}
	}
	return true
}
