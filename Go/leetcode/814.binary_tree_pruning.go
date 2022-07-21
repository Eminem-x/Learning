/**
 * Definition for a binary tree node.
 * type TreeNode struct {
 *     Val int
 *     Left *TreeNode
 *     Right *TreeNode
 * }
 */
func pruneTree(root *TreeNode) *TreeNode {
    if root == nil {
        return nil
    }

    root.Left = pruneTree(root.Left)
    root.Right = pruneTree(root.Right)

    l, r := root.Left, root.Right

    if l != nil || r != nil || root.Val == 1 {
        return root
    }

    return nil
}
