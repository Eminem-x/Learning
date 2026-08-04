package infra

import (
	"context"
	"fmt"
	"sort"

	orderapp "dddshop/internal/order/app"
	"dddshop/internal/order/domain"
)

// OrderQueryService 是 CQRS 的**读侧实现**。
//
// 注意它做了什么：直接读存储、直接拼 DTO。
//   - 不经过 domain.Order 聚合
//   - 不经过 Repository
//   - 不需要事务
//
// 在真实项目里这里就是一句 SQL：
//
//	SELECT id, buyer_id, status, total_cents, item_count FROM orders WHERE buyer_id=? ...
//
// 或者一次 ES 查询。读侧允许「不 DDD」，这是刻意的松绑：
// 硬要用聚合做查询，是 DDD 项目性能崩掉的头号原因。
type OrderQueryService struct {
	repo *OrderRepo // 演示用；真实项目这里是 *sql.DB / *elastic.Client
}

func NewOrderQueryService(r *OrderRepo) *OrderQueryService {
	return &OrderQueryService{repo: r}
}

func (q *OrderQueryService) ListByBuyer(ctx context.Context, buyerID string) ([]orderapp.OrderSummary, error) {
	var out []orderapp.OrderSummary
	for _, s := range q.repo.snapshots() {
		if s.BuyerID != buyerID {
			continue
		}
		out = append(out, orderapp.OrderSummary{
			OrderID:   s.ID,
			BuyerID:   s.BuyerID,
			Status:    domain.Status(s.Status).String(),
			Total:     fmt.Sprintf("%d.%02d %s", s.Cents/100, s.Cents%100, s.Currency),
			ItemCount: len(s.Items),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OrderID < out[j].OrderID })
	return out, nil
}
