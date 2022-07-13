func asteroidCollision(asteroids []int) []int {
    stack, idx := make([]int, 0), -1

    for _, v := range asteroids {
        if len(stack) == 0 {
            idx++
            stack = append(stack, v)
        } else {
            flag := false
            // 只有当栈中正向，实际反向才会碰撞
            for ;idx >= 0 && stack[idx] > 0 && v < 0; {
                if stack[idx] < -v {
                    idx--
                    stack = stack[:idx + 1]
                } else if stack[idx] == -v {
                    idx--
                    stack = stack[:idx + 1]
                    flag = true
                    break
                } else {
                    flag = true
                    break
                }
            }
            // 没有被销毁
            if !flag {
                idx++
                stack = append(stack, v)
            }
        }
    }

    return stack[:idx + 1]
}
