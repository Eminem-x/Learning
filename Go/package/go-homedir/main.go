// @Title go-homedir
// @Description get user's dir
// @Author Yuanhao
// @Update 2022-08-21
package main

import (
	"fmt"
	"log"
	"os/user"

	"github.com/mitchellh/go-homedir"
)

// 源码     https://github.com/mitchellh/go-homedir
// 博客     https://darjun.github.io/2020/01/14/godailylib/go-homedir/
// CGO      https://www.yisu.com/zixun/126811.html
// 交叉编译 https://blog.csdn.net/caoshangpa/article/details/79076813
// 解决办法 https://zhuanlan.zhihu.com/p/338891206
func main() {
	originDir()
	goHomedir()
}

func originDir() {
	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Home dir:", u.HomeDir)
}

func goHomedir() {
	// homedir.DisableCache = false

	dir, err := homedir.Dir()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Home dir:", dir)

	dir = "~/go/src"
	expandedDir, err := homedir.Expand(dir)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Expand of %s is: %s\n", dir, expandedDir)
}
