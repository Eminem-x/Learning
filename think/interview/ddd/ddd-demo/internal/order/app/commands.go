package app

import (
	"context"

	"dddshop/internal/order/domain"
)

// ---------------------------------------------------------------------------
// 支付
// ---------------------------------------------------------------------------

type PayOrderCmd struct {
	OrderID domain.OrderID
}

type PayOrderHandler struct {
	Tx     TxManager
	Repo   domain.Repository
	Events EventPublisher
	Clock  Clock
}

func (h *PayOrderHandler) Handle(ctx context.Context, cmd PayOrderCmd) error {
	return h.Tx.Do(ctx, func(ctx context.Context) error {
		o, err := h.Repo.FindByID(ctx, cmd.OrderID)
		if err != nil {
			return err
		}
		if err := o.Pay(h.Clock.Now()); err != nil { // ← 全场唯一一行业务逻辑，且在领域层
			return err
		}
		if err := h.Repo.Save(ctx, o); err != nil {
			return err
		}
		return h.Events.AppendAll(ctx, o.PullEvents())
	})
}

// ---------------------------------------------------------------------------
// 取消
// ---------------------------------------------------------------------------

type CancelOrderCmd struct {
	OrderID domain.OrderID
	Reason  string
}

// CancelOrderHandler 是「标准应用服务长什么样」的模板。
//
// 数一数这个函数里的代码：加载 → 调行为 → 保存 → 发事件。
// 没有一个 if 是业务规则（`已发货不可取消` 在 domain.Order.Cancel 里）。
// 这就是「应用层要薄」的具象含义。
type CancelOrderHandler struct {
	Tx     TxManager
	Repo   domain.Repository
	Events EventPublisher
	Clock  Clock
}

func (h *CancelOrderHandler) Handle(ctx context.Context, cmd CancelOrderCmd) error {
	return h.Tx.Do(ctx, func(ctx context.Context) error {
		o, err := h.Repo.FindByID(ctx, cmd.OrderID)
		if err != nil {
			return err
		}
		if err := o.Cancel(cmd.Reason, h.Clock.Now()); err != nil {
			return err
		}
		if err := h.Repo.Save(ctx, o); err != nil {
			return err // 乐观锁冲突会在这里（准确说是在事务提交时）冒出来
		}
		// ★ 与订单状态变更**同一个事务**写 outbox：
		//   要么「订单已取消 + 待发事件」一起落，要么都不落。
		//   这就是双写原子性问题的标准解法。
		return h.Events.AppendAll(ctx, o.PullEvents())
	})
}

// ---------------------------------------------------------------------------
// 发货（演示状态机的另一条边，无事件）
// ---------------------------------------------------------------------------

type ShipOrderHandler struct {
	Tx    TxManager
	Repo  domain.Repository
	Clock Clock
}

func (h *ShipOrderHandler) Handle(ctx context.Context, id domain.OrderID) error {
	return h.Tx.Do(ctx, func(ctx context.Context) error {
		o, err := h.Repo.FindByID(ctx, id)
		if err != nil {
			return err
		}
		if err := o.Ship(h.Clock.Now()); err != nil {
			return err
		}
		return h.Repo.Save(ctx, o)
	})
}
