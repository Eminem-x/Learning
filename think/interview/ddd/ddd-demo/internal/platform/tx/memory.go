// Package tx 提供事务抽象。
//
// 为什么放在 platform 而不是 domain？
// 因为「事务」是**技术概念**，领域专家不会跟你说「这里要开个事务」。
// 领域层必须对事务无感知；事务边界由**应用层**划定。
package tx

import (
	"context"
	"errors"
	"sync"
)

var ErrNestedTx = errors.New("tx: 不支持嵌套事务")

// Manager 是应用层依赖的「出站端口」。
// 真实项目里的实现是 *sql.Tx / gorm 的 Transaction，这里用内存版，
// 目的是让 demo 不依赖任何数据库也能演示「事务边界」和「回滚」。
type Manager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// UnitOfWork 工作单元：收集本次事务里所有待落盘的写操作。
//
// 每个写操作拆成两段：
//   - check：能否写（例如乐观锁版本比对）
//   - apply：真正写内存
//
// 提交时**先跑完所有 check，全部通过才跑 apply** —— 这是 2PC 的玩具版，
// 保证「要么全写，要么一个都不写」。
type UnitOfWork struct {
	ops []operation
}

type operation struct {
	check func() error
	apply func()
}

// Stage 把一个写操作登记进工作单元（此刻还没真正写）。
func (u *UnitOfWork) Stage(check func() error, apply func()) {
	u.ops = append(u.ops, operation{check: check, apply: apply})
}

func (u *UnitOfWork) commit() error {
	for _, op := range u.ops { // 阶段一：全部校验
		if op.check == nil {
			continue
		}
		if err := op.check(); err != nil {
			return err // 任何一个不通过 => 整个事务失败，没有任何 apply 被执行
		}
	}
	for _, op := range u.ops { // 阶段二：全部生效
		op.apply()
	}
	return nil
}

// Memory 内存事务管理器。
//
// 注意：每个「限界上下文」应该持有**自己独立的** Manager 实例。
// 这是刻意的设计——它在代码层面表达了 DDD 的硬约束：
// **跨限界上下文不存在共享事务**，跨 BC 只能最终一致。
type Memory struct {
	mu sync.Mutex
}

func NewMemory() *Memory { return &Memory{} }

type uowKey struct{ m *Memory }

// Do 划定一个事务边界。fn 返回 error => 全部 staged 写入被丢弃（回滚）。
func (m *Memory) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if m.UoW(ctx) != nil {
		return ErrNestedTx
	}
	m.mu.Lock() // 内存版用全局锁模拟「串行化」隔离级别
	defer m.mu.Unlock()

	u := &UnitOfWork{}
	if err := fn(context.WithValue(ctx, uowKey{m}, u)); err != nil {
		return err // ← 回滚：u 里 staged 的写操作直接被丢弃，从未 apply
	}
	return u.commit()
}

// UoW 从 ctx 里取出当前事务的工作单元；不在事务里返回 nil。
// Repository 实现用它来判断「该 staged 还是直接写」。
func (m *Memory) UoW(ctx context.Context) *UnitOfWork {
	u, _ := ctx.Value(uowKey{m}).(*UnitOfWork)
	return u
}
