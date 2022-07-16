type MovingAverage struct {
    size int
    arr []int
}

func Constructor(size int) MovingAverage {
    return MovingAverage { size: size, arr: make([]int, 0) }
}

func (this *MovingAverage) Next(val int) float64 {
    var sum int
    if len(this.arr) == this.size {
        this.arr = this.arr[1:]
    }
    // 这里会自动扩充 cap
    this.arr = append(this.arr, val)
    for _, v := range this.arr {
        sum += v
    }
    return float64(sum) / float64(len(this.arr))
}
