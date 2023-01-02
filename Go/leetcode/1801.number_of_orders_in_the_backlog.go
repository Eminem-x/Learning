import "container/heap"

func getNumberOfBacklogOrders(orders [][]int) int {
	// 小根堆
	var sell, buy hp
	for _, v := range orders {
		price, amount, orderType := v[0], v[1], v[2]
		// 如果当前订单是采购订单 buy
		if orderType == 0 {
			// 循环匹配
			for amount > 0 && len(sell) > 0 && sell[0].price <= price {
				q := heap.Pop(&sell).(pair)
				x, y := q.price, q.amount
				if amount >= y {
					amount -= y
				} else {
					heap.Push(&sell, pair{x, y - amount})
					amount = 0
				}
			}
			// 如果未完全匹配, 积压采购订单
			if amount > 0 {
				// 维持小根堆特性, -price
				heap.Push(&buy, pair{-price, amount})
			}
		}
		// 如果当前订单是销售订单 sell
		if orderType == 1 {
			for amount > 0 && len(buy) > 0 && -buy[0].price >= price {
				q := heap.Pop(&buy).(pair)
				x, y := q.price, q.amount
				if amount >= y {
					amount -= y
				} else {
					heap.Push(&buy, pair{x, y - amount})
					amount = 0
				}
			}
			if amount > 0 {
				heap.Push(&sell, pair{price, amount})
			}
		}
	}

	// 计算 amount 总数, 取模
	const mod = 1e9 + 7
	var ans int
	for i := 0; i < sell.Len(); i++ {
		ans += sell[i].amount
		ans %= mod
	}
	for i := 0; i < buy.Len(); i++ {
		ans += buy[i].amount
		ans %= mod
	}
	return ans
}

// 完成 heap 的接口实现
type pair struct {
	price  int
	amount int
}

type hp []pair

func (h hp) Len() int {
	return len(h)
}

func (h hp) Less(i, j int) bool {
	return h[i].price < h[j].price
}

func (h hp) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}
func (h *hp) Push(v interface{}) {
	*h = append(*h, v.(pair))
}
func (h *hp) Pop() interface{} {
	a := *h
	v := a[len(a)-1]
	*h = a[:len(a)-1]
	return v
}
