// Package infra 是订单 BC 的**基础设施层**（六边形架构里的「适配器」）。
//
// 这里是唯一允许出现 MySQL / Redis / MQ / 第三方 SDK 的地方。
// 领域层定义接口，这里提供实现 —— 依赖倒置的落点。
package infra

import (
	"context"
	"sync"

	"dddshop/internal/order/domain"
	"dddshop/internal/platform/tx"
)

// OrderRepo 用 map 冒充数据库，实现 domain.Repository。
//
// 换成真 MySQL 要改的只有这个文件：
// Snapshot -> INSERT/UPDATE orders + order_items，Rehydrate <- SELECT。
// 领域层、应用层一行不用动。
type OrderRepo struct {
	txm *tx.Memory

	mu   sync.RWMutex
	rows map[domain.OrderID]domain.Snapshot
}

func NewOrderRepo(txm *tx.Memory) *OrderRepo {
	return &OrderRepo{txm: txm, rows: map[domain.OrderID]domain.Snapshot{}}
}

func (r *OrderRepo) FindByID(ctx context.Context, id domain.OrderID) (*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snap, ok := r.rows[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return domain.Rehydrate(snap), nil // 从「表结构」重建聚合
}

// Save 整个聚合一起写，并做**乐观锁**版本比对。
//
// 乐观锁的语义：我加载时看到的是 v3，写回去时如果库里已经是 v4，
// 说明有人在我操作期间改过 —— 我的修改基于过期数据，必须失败重来。
//
// 为什么用乐观锁而不是 SELECT ... FOR UPDATE？
// 因为聚合的修改是「读-改-写」，悲观锁要跨越整个业务处理时间持锁，
// 在高并发下把吞吐锁死；乐观锁只在提交瞬间检查，冲突时重试即可。
func (r *OrderRepo) Save(ctx context.Context, o *domain.Order) error {
	snap := o.Snapshot()

	check := func() error {
		r.mu.RLock()
		defer r.mu.RUnlock()
		cur, ok := r.rows[o.ID()]
		if !ok {
			if snap.Version != 0 {
				return domain.ErrNotFound // 记录没了但版本非 0：被删了
			}
			return nil // 新建
		}
		if cur.Version != snap.Version {
			return domain.ErrConflict // ★ 并发冲突
		}
		return nil
	}
	apply := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		snap.Version++
		r.rows[o.ID()] = snap
		o.SetVersion(snap.Version) // 回写版本号（Go 没有 ORM 脏检查，只能显式做）
	}

	// 在事务里 => 登记进工作单元，等提交时统一 check + apply（保证原子）
	if uow := r.txm.UoW(ctx); uow != nil {
		uow.Stage(check, apply)
		return nil
	}
	// 不在事务里 => 立即执行
	if err := check(); err != nil {
		return err
	}
	apply()
	return nil
}

// snapshots 给同包的查询侧使用（模拟「读侧直接查库」）。
func (r *OrderRepo) snapshots() []domain.Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.Snapshot, 0, len(r.rows))
	for _, s := range r.rows {
		out = append(out, s)
	}
	return out
}
