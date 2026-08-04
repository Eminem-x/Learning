package domain

import (
	"errors"
	"fmt"
)

var (
	ErrCurrencyMismatch = errors.New("币种不一致")
	ErrNegativeAmount   = errors.New("金额不能为负")
	ErrEmptyCurrency    = errors.New("币种不能为空")
)

// Money 是**值对象（Value Object）**的标准样板，四个特征全占：
//
//  1. 无唯一标识 —— 两张 100 元钞票在业务上完全等价
//  2. 不可变 —— 所有方法返回新值，绝不修改自身
//  3. 值相等 —— Equals 比的是全部属性，不是 ID
//  4. 自我校验 —— 字段**不导出**，外部只能通过 NewMoney 构造，
//     于是「金额非负 / 币种非空」这两条规则在全系统只写一遍，无法绕过
//
// 顺带解决「基本类型偏执」：签名里写 Money 而不是 float64，
// 传参传反、单位搞错（元 vs 分）这类 bug 在编译期就被挡住。
type Money struct {
	cents    int64 // 以「分」为单位存储，杜绝浮点误差（0.1+0.2 != 0.3）
	currency string
}

func NewMoney(cents int64, currency string) (Money, error) {
	if cents < 0 {
		return Money{}, ErrNegativeAmount
	}
	if currency == "" {
		return Money{}, ErrEmptyCurrency
	}
	return Money{cents: cents, currency: currency}, nil
}

// MustMoney 仅供测试/常量场景使用。
func MustMoney(cents int64, currency string) Money {
	m, err := NewMoney(cents, currency)
	if err != nil {
		panic(err)
	}
	return m
}

// Add 返回新的 Money —— 不可变性的体现：m 本身不会被改。
func (m Money) Add(o Money) (Money, error) {
	if m.currency != o.currency {
		return Money{}, ErrCurrencyMismatch
	}
	return Money{cents: m.cents + o.cents, currency: m.currency}, nil
}

func (m Money) MulInt(n int) Money {
	return Money{cents: m.cents * int64(n), currency: m.currency}
}

// Equals 值相等：全部属性相同才相等（对比实体是按 ID 判等）。
func (m Money) Equals(o Money) bool {
	return m.cents == o.cents && m.currency == o.currency
}

func (m Money) Cents() int64     { return m.cents }
func (m Money) Currency() string { return m.currency }

func (m Money) String() string {
	return fmt.Sprintf("%d.%02d %s", m.cents/100, m.cents%100, m.currency)
}
