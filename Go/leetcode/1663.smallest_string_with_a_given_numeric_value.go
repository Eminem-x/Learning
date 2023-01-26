func getSmallestString(n int, k int) string {
	s := []byte{}
	for i := n - 1; i >= 1; i-- {
		for j := 1; j <= 26; j++ {
			if (k - j) <= 26*i {
				// fmt.Printf("%d %d\n", k - j + 1, 26 * i)
				k -= j
				s = append(s, 'a'+byte(j-1))
				break
			}
		}
	}
	s = append(s, 'a'+byte(k)-1)
	return string(s)
}
