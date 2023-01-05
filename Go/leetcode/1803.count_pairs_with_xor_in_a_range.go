func countPairs(nums []int, low int, high int) int {
	var t trie
	res := 0
	for _, v := range nums {
		res += t.search(v, high+1) - t.search(v, low)
		t.insert(v)
	}
	return res
}

type trie struct {
	child [2]*trie
	cnt   int
}

func (t *trie) insert(x int) {
	cur := t
	for i := 15; i >= 0; i-- {
		bit := (x >> i) & 1
		if cur.child[bit] == nil {
			cur.child[bit] = &trie{}
		}
		cur = cur.child[bit]
		cur.cnt++
	}
}

func (t *trie) search(x int, limit int) int {
	cur := t
	res := 0
	for i := 15; i >= 0; i-- {
		if cur == nil {
			break
		}
		bit := (x >> i) & 1
		if (limit>>i)&1 == 1 {
			if cur.child[bit] != nil {
				res += cur.child[bit].cnt
			}
			cur = cur.child[1-bit]
		} else {
			cur = cur.child[bit]
		}
	}
	return res
}
