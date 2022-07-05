type MyCalendar struct {
    intervals [][]int
}

func Constructor() MyCalendar {
    return MyCalendar { intervals: make([][]int, 0) }
}

func (this *MyCalendar) Book(start int, end int) bool {
    for i := 0; i < len(this.intervals); i++ {
        t := this.intervals
        if (start >= t[i][0] && start < t[i][1]) || (end > t[i][0] && end <= t[i][1]) {
            return false
        }
        if start <= t[i][0] && end >= t[i][1] {
            return false
        }
    }
    this.intervals = append(this.intervals, []int{start, end})
    return true
}
