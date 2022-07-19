type MyCalendarTwo struct {
    *redblacktree.Tree
}

func Constructor() MyCalendarTwo {
    return MyCalendarTwo{redblacktree.NewWithIntComparator()}
}

func (c MyCalendarTwo) add(key, val int) {
    if v, ok := c.Get(key); ok {
        c.Put(key, v.(int) + val)
    } else {
        c.Put(key, val)
    }
}

func (c *MyCalendarTwo) Book(start int, end int) bool {
    c.add(start, 1)
    c.add(end, -1)
    maxBook := 0
    it := c.Iterator()
    for it.Next() {
        maxBook += it.Value().(int)
        if maxBook > 2 {
            c.add(start, -1)
            c.add(end, 1)
            return false
        }
    }
    return true
}


/**
* Your MyCalendarTwo object will be instantiated and called as such:
* obj := Constructor();
* param_1 := obj.Book(start,end);
*/
