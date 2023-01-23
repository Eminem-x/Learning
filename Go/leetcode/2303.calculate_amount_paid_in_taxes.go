func calculateTax(brackets [][]int, income int) float64 {
	var t int
	var res float64
	for _, bracket := range brackets {
		if income > bracket[0] {
			res += float64(bracket[0]-t) * float64(bracket[1]) / 100
			t = bracket[0]
		} else {
			res += float64(income-t) * float64(bracket[1]) / 100
			return res
		}
	}
	return res
}
