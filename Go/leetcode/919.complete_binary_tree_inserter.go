/**
 * Definition for a binary tree node.
 * type TreeNode struct {
 *     Val int
 *     Left *TreeNode
 *     Right *TreeNode
 * }
 */
type CBTInserter struct {
	arr []*TreeNode
}

func Constructor(root *TreeNode) CBTInserter {
	queue, t := make([]*TreeNode, 0), make([]*TreeNode, 0)
	queue = append(queue, root)

	// 层序遍历,按照数组存储
	for len(queue) > 0 {
		size := len(queue)
		for i := 0; i < size; i++ {
			node := queue[i]
			t = append(t, &TreeNode{Val: node.Val})
			if node.Left != nil {
				queue = append(queue, node.Left)
			}
			if node.Right != nil {
				queue = append(queue, node.Right)
			}
		}
		queue = queue[size:]
	}

	return CBTInserter{arr: t}
}

func (this *CBTInserter) Insert(val int) int {
	this.arr = append(this.arr, &TreeNode{Val: val})
	return this.arr[len(this.arr)/2-1].Val
}

func (this *CBTInserter) Get_root() *TreeNode {
	root := &TreeNode{Val: this.arr[0].Val}

	queue := make([]*TreeNode, 0)
	queue = append(queue, root)

	// 逆向层序遍历,构造完全二叉树
	var cnt int
	for len(queue) > 0 {
		size := len(queue)
		for i := 0; i < size; i++ {
			node := queue[i]
			if (cnt+i)*2+1 < len(this.arr) {
				node.Left = this.arr[(cnt+i)*2+1]
				queue = append(queue, node.Left)
			}
			if (cnt+i)*2+2 < len(this.arr) {
				node.Right = this.arr[(cnt+i)*2+2]
				queue = append(queue, node.Right)
			}
		}
		queue, cnt = queue[size:], cnt+size
	}

	return root
}

/**
 * Your CBTInserter object will be instantiated and called as such:
 * obj := Constructor(root);
 * param_1 := obj.Insert(val);
 * param_2 := obj.Get_root();
 */
