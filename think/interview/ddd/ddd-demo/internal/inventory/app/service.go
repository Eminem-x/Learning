// Package app 是库存 BC 的应用层。
package app

import (
	"context"

	"dddshop/internal/inventory/domain"
	"dddshop/internal/platform/tx"
)

type Line struct {
	SkuID string
	Qty   int
}

type Service struct {
	Tx   tx.Manager
	Repo domain.Repository
	Logf func(format string, args ...any)
}

// Reserve 预留多个 SKU 的库存。
//
// ★ 这个方法是「一个事务只改一个聚合」原则的实战演示。
//
// 一次预留涉及 N 个 SKU = N 个 Stock 聚合。偷懒的做法是开一个大事务
// 把 N 个聚合一起改掉；DDD 的做法是**每个聚合一个事务**，
// 失败时用**补偿（Saga）**把已成功的部分退回去。
//
// 为什么值得这么麻烦？
//   - 大事务持锁时间长，热点 SKU 上直接把吞吐打穿
//   - 一旦 Stock 将来拆到不同分片/不同库，大事务根本无法存在
//
// 代价：中间态是可见的（有一瞬间部分 SKU 已预留、部分未预留）。
// 这就是最终一致性的本质 —— 用「短暂的不一致」换「可伸缩」。
func (s *Service) Reserve(ctx context.Context, orderID string, lines []Line) error {
	var done []string

	for _, l := range lines {
		line := l
		err := s.Tx.Do(ctx, func(ctx context.Context) error { // ← 一个 SKU 一个事务
			st, err := s.Repo.FindBySKU(ctx, line.SkuID)
			if err != nil {
				return err
			}
			if err := st.Reserve(orderID, line.Qty); err != nil { // 业务规则在聚合里
				return err
			}
			return s.Repo.Save(ctx, st)
		})
		if err != nil {
			s.logf("        [库存] %s 预留失败(%v)，补偿回滚已预留的 %v", line.SkuID, err, done)
			s.release(ctx, orderID, done) // ★ Saga 补偿
			return err
		}
		done = append(done, line.SkuID)
	}
	return nil
}

// Release 释放预留。幂等由 Stock 聚合内部保证，这里只负责编排。
func (s *Service) Release(ctx context.Context, orderID string, skuIDs []string) error {
	s.release(ctx, orderID, skuIDs)
	return nil
}

func (s *Service) release(ctx context.Context, orderID string, skuIDs []string) {
	for _, sku := range skuIDs {
		skuID := sku
		err := s.Tx.Do(ctx, func(ctx context.Context) error {
			st, err := s.Repo.FindBySKU(ctx, skuID)
			if err != nil {
				return err
			}
			if !st.Release(orderID) {
				// 没有预留记录 —— 可能是重复投递，也可能早就释放过了。
				// 幂等场景，正常返回，不要报错更不要重试。
				s.logf("        [库存] %s 无 %s 的预留记录 → 幂等跳过（重复投递被安全吸收）", skuID, orderID)
				return nil
			}
			s.logf("        [库存] %s 释放了 %s 的预留", skuID, orderID)
			return s.Repo.Save(ctx, st)
		})
		if err != nil {
			// 补偿失败在真实系统里要落「补偿失败表 + 告警 + 人工/定时兜底」，
			// 绝不能静默吞掉。
			s.logf("        [库存] 释放 %s/%s 失败: %v（真实系统应告警 + 定时对账兜底）", skuID, orderID, err)
		}
	}
}

func (s *Service) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
	}
}

// Available 查询可用量（读侧，直接读，不走聚合行为）。
func (s *Service) Available(ctx context.Context, skuID string) (int, error) {
	st, err := s.Repo.FindBySKU(ctx, skuID)
	if err != nil {
		return 0, err
	}
	return st.Available(), nil
}
