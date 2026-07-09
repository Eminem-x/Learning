// 最小可运行 B+ 树:演示三件核心机制
//  1. 节点满则分裂,并向上传播(root 分裂时树长高一层)
//  2. 叶子用 next 串成链表(这里做成双向,带 prev)
//  3. RangeScan:定位起点叶子后,顺链表往右扫,不回树
//
// 对照笔记:think/interview/tree-structures-bplus-btree-rbtree-20260708.md 第五/六/八节
// 运行:go run main.go
package main

import (
	"fmt"
	"sort"
	"strings"
)

// order = 阶 m:每个节点最多 m 个孩子、m-1 个 key。
// 现实里 InnoDB 的 m≈1170(16KB 页 / 14B 路标);这里取 4 只为让分裂肉眼可见。
const order = 4

type node struct {
	isLeaf   bool
	keys     []int    // 叶子:真实 key;内部:分界路标(separator)
	values   []string // 仅叶子:与 keys 一一对应的数据
	children []*node  // 仅内部:len == len(keys)+1
	next     *node    // 仅叶子:右邻叶子(链表)
	prev     *node    // 仅叶子:左邻叶子(双向,支持 ORDER BY DESC)
}

type BPlusTree struct{ root *node }

func New() *BPlusTree { return &BPlusTree{root: &node{isLeaf: true}} }

// ---------- 查询:一路下降到叶子,深度恒定 ----------

func (t *BPlusTree) Search(key int) (string, bool) {
	n := t.root
	for !n.isLeaf {
		n = n.children[route(n, key)]
	}
	i := sort.SearchInts(n.keys, key)
	if i < len(n.keys) && n.keys[i] == key {
		return n.values[i], true
	}
	return "", false
}

// route:在内部节点里,key 该往第几个孩子拐。
// child[i] 负责区间 [keys[i-1], keys[i]);最右孩子敞口无上限(对应笔记 6.1)。
func route(n *node, key int) int {
	i := 0
	for i < len(n.keys) && key >= n.keys[i] {
		i++
	}
	return i
}

// ---------- 范围查询:B+ 树的杀手锏 ----------

// RangeScan 返回 [lo, hi] 内的所有记录。先定位到起点叶子,再顺 next 链表扫。
func (t *BPlusTree) RangeScan(lo, hi int) []string {
	n := t.root
	for !n.isLeaf { // 定位起点叶子
		n = n.children[route(n, lo)]
	}
	var out []string
	for n != nil { // ★ 顺叶子链表往右,不回树
		for i, k := range n.keys {
			if k < lo {
				continue
			}
			if k > hi {
				return out
			}
			out = append(out, fmt.Sprintf("%d=%s", k, n.values[i]))
		}
		n = n.next
	}
	return out
}

// ---------- 插入:满则分裂,向上传播 ----------

func (t *BPlusTree) Insert(key int, value string) {
	up, right := insert(t.root, key, value)
	if right != nil { // 根分裂了 → 新建根,树长高一层
		t.root = &node{keys: []int{up}, children: []*node{t.root, right}}
	}
}

// 返回:若子树发生分裂,返回上移的 separator 和新右节点;否则 (0, nil)。
func insert(n *node, key int, value string) (int, *node) {
	if n.isLeaf {
		insertIntoLeaf(n, key, value)
		if len(n.keys) < order { // 未溢出(最多 order-1 个 key)
			return 0, nil
		}
		return splitLeaf(n)
	}
	i := route(n, key)
	up, right := insert(n.children[i], key, value)
	if right == nil {
		return 0, nil
	}
	insertIntoInternal(n, i, up, right) // 孩子分裂了,把 separator 插进本节点
	if len(n.keys) < order {
		return 0, nil
	}
	return splitInternal(n)
}

func insertIntoLeaf(n *node, key int, value string) {
	i := sort.SearchInts(n.keys, key)
	if i < len(n.keys) && n.keys[i] == key { // 已存在 → 更新
		n.values[i] = value
		return
	}
	n.keys = insertInt(n.keys, i, key)
	n.values = insertStr(n.values, i, value)
}

// splitLeaf:分裂叶子,separator = 右边第一个 key(复制上移,叶子仍保留它)。
func splitLeaf(n *node) (int, *node) {
	mid := len(n.keys) / 2
	right := &node{isLeaf: true}
	right.keys = append(right.keys, n.keys[mid:]...)
	right.values = append(right.values, n.values[mid:]...)
	n.keys = n.keys[:mid:mid]
	n.values = n.values[:mid:mid]
	// 维护双向叶子链表:n <-> right <-> 原 n.next
	right.next, right.prev = n.next, n
	if n.next != nil {
		n.next.prev = right
	}
	n.next = right
	return right.keys[0], right
}

// splitInternal:分裂内部节点,中间 key 上移(移走,不复制)。
func splitInternal(n *node) (int, *node) {
	mid := len(n.keys) / 2
	up := n.keys[mid]
	right := &node{}
	right.keys = append(right.keys, n.keys[mid+1:]...)
	right.children = append(right.children, n.children[mid+1:]...)
	n.keys = n.keys[:mid:mid]
	n.children = n.children[: mid+1 : mid+1]
	return up, right
}

func insertIntoInternal(n *node, i, up int, right *node) {
	n.keys = insertInt(n.keys, i, up)
	// right 作为分界点 up 的右孩子,插到 children[i+1]
	n.children = append(n.children, nil)
	copy(n.children[i+2:], n.children[i+1:])
	n.children[i+1] = right
}

// ---------- 小工具:切片指定位置插入 ----------

func insertInt(s []int, i, v int) []int {
	s = append(s, 0)
	copy(s[i+1:], s[i:])
	s[i] = v
	return s
}

func insertStr(s []string, i int, v string) []string {
	s = append(s, "")
	copy(s[i+1:], s[i:])
	s[i] = v
	return s
}

// ---------- 可视化:按层打印 + 树高 ----------

func (t *BPlusTree) String() string {
	var sb strings.Builder
	level := []*node{t.root}
	depth := 0
	for len(level) > 0 {
		var next []*node
		var parts []string
		for _, n := range level {
			parts = append(parts, fmt.Sprintf("%v", n.keys))
			if !n.isLeaf {
				next = append(next, n.children...)
			}
		}
		tag := "内部"
		if level[0].isLeaf {
			tag = "叶子"
		}
		fmt.Fprintf(&sb, "第%d层(%s): %s\n", depth, tag, strings.Join(parts, "  "))
		level, depth = next, depth+1
	}
	return sb.String()
}

func main() {
	t := New()
	// 顺序插入 1..20:对普通 BST 会退化成链表,B+ 树靠分裂始终保持矮胖
	for i := 1; i <= 20; i++ {
		t.Insert(i, fmt.Sprintf("v%d", i))
		// fmt.Print(t)
	}

	fmt.Println("=== 树形(order=4) ===")
	fmt.Print(t)

	fmt.Println("=== 等值查询 ===")
	for _, k := range []int{7, 15, 99} {
		v, ok := t.Search(k)
		fmt.Printf("Search(%d) = %q, found=%v\n", k, v, ok)
	}

	fmt.Println("\n=== 范围查询 [6, 13](顺叶子链表扫,不回树)===")
	fmt.Println(t.RangeScan(6, 13))

	fmt.Println("\n=== 更新已存在的 key ===")
	t.Insert(10, "v10-updated")
	v, _ := t.Search(10)
	fmt.Printf("Search(10) = %q\n", v)
}
