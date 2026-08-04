// Package domain 是**库存限界上下文**的领域层。
//
// 划重点：为什么库存要单独一个 BC，而不是塞进订单聚合？
//
//  1. 不变性不同：订单管「状态机 + 总额」，库存管「可用量 >= 0」。
//     两者不需要强一致地绑在一起。
//  2. 聚合要小：把库存塞进订单，爆款 SKU 的并发扣减会把订单写入拖垮。
//  3. 生命周期不同：库存是长期存在的，订单是一次性的。
//
// 于是「不超卖」这条规则由**库存聚合自己**保证，订单只管下单。
// 两者之间用最终一致（领域事件）连接。
package domain

import (
	"context"
	"errors"
	"sort"
)

var (
	ErrInsufficient = errors.New("库存不足")
	ErrInvalidQty   = errors.New("数量必须大于 0")
	ErrSkuNotFound  = errors.New("SKU 不存在")
	ErrConflict     = errors.New("并发修改冲突，请重试")
)

// Stock 是库存 BC 的聚合根，粒度 = 单个 SKU。
//
// 粒度选择本身就是考点：
//   - 粒度按 SKU  => 聚合极小，不同 SKU 的扣减互不阻塞（正确）
//   - 粒度按仓库  => 一个仓库所有 SKU 抢一把锁，热点直接打爆（错误）
type Stock struct {
	skuID string
	total int // 总库存

	// reservations: orderID -> 预留数量
	//
	// ★ 这个 map 是全篇最值得琢磨的设计。
	// 它让「幂等」不再是外挂的去重表，而是**领域模型自带的语义**：
	// 「同一个订单只能占一次坑」本身就是业务规则。
	// MQ 重复投递 => 重复调用 Reserve/Release => 天然无副作用。
	reservations map[string]int

	version int
}

func NewStock(skuID string, total int) *Stock {
	return &Stock{skuID: skuID, total: total, reservations: map[string]int{}}
}

// Available 可用量 = 总量 - 已预留。这是本聚合要守住的不变性核心。
func (s *Stock) Available() int {
	used := 0
	for _, q := range s.reservations {
		used += q
	}
	return s.total - used
}

// Reserve 预留库存（下单占坑）。
func (s *Stock) Reserve(orderID string, qty int) error {
	if qty <= 0 {
		return ErrInvalidQty
	}
	if _, ok := s.reservations[orderID]; ok {
		return nil // 幂等：这个订单已经占过坑了，重复调用什么都不做
	}
	if s.Available() < qty {
		return ErrInsufficient // ★ 不变性：可用量永远不会变成负数 —— 这就是「不超卖」
	}
	s.reservations[orderID] = qty
	return nil
}

// Release 释放预留（订单取消/超时）。返回是否真的释放了。
// 幂等：没有该订单的预留记录就静默返回 false，不报错。
func (s *Stock) Release(orderID string) bool {
	if _, ok := s.reservations[orderID]; !ok {
		return false
	}
	delete(s.reservations, orderID)
	return true
}

func (s *Stock) SkuID() string    { return s.skuID }
func (s *Stock) Total() int       { return s.total }
func (s *Stock) Version() int     { return s.version }
func (s *Stock) SetVersion(v int) { s.version = v }

// ---------------------------------------------------------------------------
// 持久化快照（同订单 BC，见 order/domain/order.go 末尾的说明）
// ---------------------------------------------------------------------------

type Reservation struct {
	OrderID string
	Qty     int
}

type Snapshot struct {
	SkuID        string
	Total        int
	Reservations []Reservation
	Version      int
}

func (s *Stock) Snapshot() Snapshot {
	rs := make([]Reservation, 0, len(s.reservations))
	for id, q := range s.reservations {
		rs = append(rs, Reservation{OrderID: id, Qty: q})
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].OrderID < rs[j].OrderID }) // 稳定顺序，便于断言
	return Snapshot{SkuID: s.skuID, Total: s.total, Reservations: rs, Version: s.version}
}

func Rehydrate(s Snapshot) *Stock {
	m := make(map[string]int, len(s.Reservations))
	for _, r := range s.Reservations {
		m[r.OrderID] = r.Qty
	}
	return &Stock{skuID: s.SkuID, total: s.Total, reservations: m, version: s.Version}
}

// Repository 端口，同样定义在领域层。
type Repository interface {
	FindBySKU(ctx context.Context, skuID string) (*Stock, error)
	Save(ctx context.Context, s *Stock) error
}
