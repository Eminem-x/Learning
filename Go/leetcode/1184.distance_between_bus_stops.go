func distanceBetweenBusStops(distance []int, start int, destination int) int {
	if destination < start {
		start, destination = destination, start
	}

	ans, total := 0, 0
	for i := 0; i < len(distance); i++ {
		total += distance[i]
	}
	for i := start; i < destination; i++ {
		ans += distance[i]
	}

	if total-ans < ans {
		ans = total - ans
	}

	return ans
}
