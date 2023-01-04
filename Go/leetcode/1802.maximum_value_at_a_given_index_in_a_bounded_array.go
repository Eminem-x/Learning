func maxValue(n int, index int, maxSum int) int {
	left, right := 1, maxSum
	for left < right {
		mid := (left + right) / 2
		// 分别计算左侧右侧值
		sum := mid + getSum(mid, index) + getSum(mid, n-index-1)
		// fmt.Println(sum)
		if sum > maxSum {
			right = mid
		} else {
			left = mid + 1
		}
	}
	// fmt.Printf("%d %d", left, right)
	sum := left + getSum(left, index) + getSum(left, n-index-1)
	if sum == maxSum {
		return left
	}
	return left - 1
}

func getSum(n int, length int) int {
	if length == 0 {
		return 0
	}

	if n > length {
		return (n - length + n - 1) * length / 2
	}

	return (length - n + 1) + (1+n-1)*(n-1)/2
}
