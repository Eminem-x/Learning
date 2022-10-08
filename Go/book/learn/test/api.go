package test

import (
	"bufio"
	"github.com/bytedance/gopkg/lang/fastrand"
	"math/rand"
	"os"
	"strings"
)

// 单元测试

// HelloTom return Tom
func HelloTom() string {
	return "Tom"
}

// 文件测试 + Mock测试

func ReadFirstLine() string {
	open, err := os.Open("log")
	defer open.Close()
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(open)
	for scanner.Scan() {
		return scanner.Text()
	}
	return ""
}

func ProcessFirstLine() string {
	line := ReadFirstLine()
	destLine := strings.ReplaceAll(line, "11", "00")
	return destLine
}

// 基准测试

var ServerIndex [10]int

func InitServerIndex() {
	for i := 0; i < 10; i++ {
		ServerIndex[i] = i + 100
	}
}

func Select() int {
	return ServerIndex[rand.Intn(10)]
}

func FastSelect() int {
	// go get github.com/bytedance/gopkg bytedance 开源方法 优化 rand 函数
	return ServerIndex[fastrand.Intn(10)]
}
