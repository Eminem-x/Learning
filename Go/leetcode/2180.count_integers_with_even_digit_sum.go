func countEven(num int) int {
	var res int
	for i := 1; i <= num; i++ {
		var sum int
		for j := i; j != 0; j /= 10 {
			sum += j % 10
		}
		if sum%2 == 0 {
			res++
		}
	}
	return res
}
