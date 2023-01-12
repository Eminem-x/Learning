func rearrangeCharacters(s string, target string) int {
	hashMap := make(map[rune]int)
	keyMap := make(map[rune]int)
	for _, v := range s {
		hashMap[v]++
	}
	for _, v := range target {
		keyMap[v]++
	}
	res := 100
	for k, v := range keyMap {
		if hashMap[k]/v < res {
			res = hashMap[k] / v
		}
	}
	return res
}
