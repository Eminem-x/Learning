func minRefuelStops(target int, startFuel int, stations [][]int) int {
    start, ans, pq := 0, 0, hp{}
    for i := 0; i < len(stations); i++ {
        // 判断当前能否到达站点
        for stations[i][0] - start > startFuel {
            if pq.Len() > 0 {
                ans++
                // 注意是 heap.Pop(&pq)
                startFuel += heap.Pop(&pq).(int)
            } else {
                // 如果到不了
                return -1
            }
        }
        // 如果已经到达终点
        if stations[i][0] >= target {
            return ans
        }
        // 更新油和站点、带上当前站点油
        startFuel -= stations[i][0] - start;
        start = stations[i][0]
        heap.Push(&pq, stations[i][1])
    }

    // 说明到了最后一站，依然没有结果
    for start + startFuel < target {
        if pq.Len() > 0 {
            ans++
            startFuel += heap.Pop(&pq).(int)
        } else {
            return -1
        }
    }
    return ans;
}

// 实现排序接口
type hp struct{ sort.IntSlice }

func (h hp) Less(i, j int) bool { 
    return h.IntSlice[i] > h.IntSlice[j] 
}

// 利用自动排序实现 Push 和 Pop
func (h *hp) Push(v interface{}) {
    h.IntSlice = append(h.IntSlice, v.(int))
}

func (h *hp) Pop() interface{} {
    fmt.Println(h.IntSlice)
    a := h.IntSlice; 
    v := a[len(a)-1];
    h.IntSlice = a[:len(a)-1];
    return v 
}
