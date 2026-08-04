// Package infra 是库存 BC 的基础设施层（适配器）。
package infra

import (
	"context"
	"sync"

	"dddshop/internal/inventory/domain"
	"dddshop/internal/platform/tx"
)

// StockRepo 用 map 冒充数据库，实现 domain.Repository。
// 换成 MySQL 只需要重写这个文件，领域层和应用层一行不动 —— 这就是依赖倒置的收益。
type StockRepo struct {
	txm *tx.Memory

	mu   sync.RWMutex
	rows map[string]domain.Snapshot
}

func NewStockRepo(txm *tx.Memory) *StockRepo {
	return &StockRepo{txm: txm, rows: map[string]domain.Snapshot{}}
}

// Seed 初始化库存（仅 demo 用，绕过应用层）。
func (r *StockRepo) Seed(skuID string, total int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[skuID] = domain.NewStock(skuID, total).Snapshot()
}

func (r *StockRepo) FindBySKU(ctx context.Context, skuID string) (*domain.Stock, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snap, ok := r.rows[skuID]
	if !ok {
		return nil, domain.ErrSkuNotFound
	}
	return domain.Rehydrate(snap), nil
}

func (r *StockRepo) Save(ctx context.Context, s *domain.Stock) error {
	snap := s.Snapshot()

	check := func() error {
		r.mu.RLock()
		defer r.mu.RUnlock()
		cur, ok := r.rows[snap.SkuID]
		if !ok {
			return domain.ErrSkuNotFound
		}
		if cur.Version != snap.Version {
			return domain.ErrConflict // 乐观锁：加载后有人改过
		}
		return nil
	}
	apply := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		snap.Version++
		r.rows[snap.SkuID] = snap
		s.SetVersion(snap.Version)
	}

	if uow := r.txm.UoW(ctx); uow != nil {
		uow.Stage(check, apply)
		return nil
	}
	if err := check(); err != nil {
		return err
	}
	apply()
	return nil
}
