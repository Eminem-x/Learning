func minOperations(nums []int, x int) int {
	hashMap := make(map[int]int)
	var sum int
	for i := len(nums) - 1; i >= 0; i-- {
		sum += nums[i]
		// fmt.Println(sum)
		hashMap[sum] = i
	}

	// 全部右后缀是否存在
	res, length := -1, len(nums)
	if idx, ok := hashMap[x]; ok {
		res = length - idx
	}

	var t int
	for i, v := range nums {
		t += v
		if t == x {
			res = getMin(res, i+1)
		}
		// fmt.Println(x - t)
		if idx, ok := hashMap[x-t]; ok && idx > i {
			// fmt.Println(idx)
			cnt := i + 1 + length - idx
			res = getMin(res, cnt)
		}
	}

	return res
}

func getMin(res, x int) int {
	if res == -1 {
		res = x
	} else {
		if x < res {
			res = x
		}
	}
	return res
}
