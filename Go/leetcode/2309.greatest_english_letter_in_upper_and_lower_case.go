func greatestLetter(s string) string {
	hashMap := make(map[rune]bool)
	for _, c := range s {
		hashMap[c] = true
	}
	// fmt.Println('A' - 'a')
	res := []byte{}
	for i := 'Z'; i >= 'A'; i-- {
		if hashMap[i] && hashMap[i+32] {
			res = append(res, byte(i))
			return string(res)
		}
	}
	return ""
}
