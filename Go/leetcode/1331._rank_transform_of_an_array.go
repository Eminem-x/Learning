func arrayRankTransform(arr []int) []int {
	nums := make([][]int, len(arr))
	for i := 0; i < len(arr); i++ {
		nums[i] = make([]int, 2)
		nums[i][0] = arr[i]
		nums[i][1] = i
	}

	sort.Slice(nums, func(i, j int) bool {
		a, b := nums[i], nums[j]
		return a[0] < b[0]
	})

	idx := 1
	for i := 0; i < len(arr); i++ {
		arr[nums[i][1]] = idx
		for j := i + 1; j < len(arr); j++ {
			if nums[i][0] == nums[j][0] {
				i++
				arr[nums[j][1]] = idx
			} else {
				break
			}
		}
		idx++
	}

	return arr
}
