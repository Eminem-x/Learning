// @Title cobra
// @Description cobra provide simple way to create modern CLI
// @Author Yuanhao
// @Update 2022-08-22
package main

import "go-cobra/cmd"

// 官方 https://github.com/spf13/cobra
// 博客 https://darjun.github.io/2020/01/17/godailylib/cobra/
func main() {
	// go build -o git main.go
	// git -h、git version -h、git version
	cmd.Execute()
}
