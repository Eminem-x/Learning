package test

import (
	"bou.ke/monkey"
	"github.com/stretchr/testify/assert"
	"testing"
)

// go test -v api_test.go api_test --cover 详细查看测试覆盖率以及测试信息

func TestHelloTom(t *testing.T) {
	output := HelloTom()
	expectOutput := "Jerry"
	if output != expectOutput {
		t.Errorf("output: %s not match expectOutput: %s", output, expectOutput)
	}
}

// go test -v api_test.go api.go -test.run TestHelloTom2 测试单个函数
func TestHelloTom2(t *testing.T) {
	output := HelloTom()
	expectOutput := "Tom"
	assert.Equal(t, expectOutput, output)
}

func TestProcessFirstLine(t *testing.T) {
	// undefined: jmpToFunctionValue, because not work on Mac with M1
	// issue: https://github.com/jenkins-x/jx/issues/2081

	monkey.Patch(ReadFirstLine, func() string {
		return "line110"
	})
	defer monkey.Unpatch(ReadFirstLine)
	firstLine := ProcessFirstLine()
	assert.Equal(t, "line000", firstLine)
}

func BenchmarkSelect(b *testing.B) {
	InitServerIndex()
	// 重置计时器，在此之前的 init 操作将不作为基准测试的范围
	b.ResetTimer()
	// 用 b 中的 N 值反复递增循环测试
	// 当测试用例函数返回时不足一秒，testing.B 中的 N 的值将按 1、2、5、10... 递增
	// 并且以递增后的值重新进行用例函数测试
	for i := 0; i < b.N; i++ {
		Select()
	}
}

func BenchmarkSelectParallel(b *testing.B) {
	InitServerIndex()
	b.ResetTimer()
	// RunParallel 多协程并发测试
	// go 原生的 rand 为了保证全局的随机性和并发安全，持有一把全局锁
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Select()
		}
	})
}

func BenchmarkFastSelectParallel(b *testing.B) {
	InitServerIndex()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			FastSelect()
		}
	})
}
