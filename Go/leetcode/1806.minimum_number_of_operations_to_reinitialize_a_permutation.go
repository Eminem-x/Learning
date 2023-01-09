func reinitializePermutation(n int) int {
	// 构造模拟数组
	nums := make([]int, n)
	for i := range nums {
		nums[i] = i
	}

	// 模拟变化数组
	for i := 1; ; i++ {
		nums = operate(nums)
		if isBegin(nums) {
			return i
		}
	}

	return 0
}

func operate(nums []int) []int {
	arr := make([]int, len(nums))
	for i := range nums {
		if i%2 == 0 {
			arr[i] = nums[i/2]
		} else {
			arr[i] = nums[len(nums)/2+(i-1)/2]
		}
	}
	return arr
}

func isBegin(nums []int) bool {
	for i, _ := range nums {
		if nums[i] != i {
			return false
		}
	}
	return true
}
