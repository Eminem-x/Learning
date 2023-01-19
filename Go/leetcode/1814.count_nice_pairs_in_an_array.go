func countNicePairs(nums []int) int {
	hashMap := make(map[int]int)
	var res int
	for _, num := range nums {
		var t int
		for x := num; x > 0; x /= 10 {
			t = t*10 + x%10
		}
		res += hashMap[num-t]
		hashMap[num-t]++
		res %= 1e9 + 7
	}
	return res
}
