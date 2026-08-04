package app

import (
	"context"

	"dddshop/internal/order/domain"
)

type PlaceLine struct {
	SkuID      string
	Name       string
	Qty        int
	PriceCents int64
}

type PlaceOrderCmd struct {
	BuyerID string
	Lines   []PlaceLine
}

// PlaceOrderHandler 下单。整个 demo 里最值得逐行读的一个应用服务。
type PlaceOrderHandler struct {
	Tx     TxManager
	Repo   domain.Repository
	Events EventPublisher
	Inv    InventoryPort
	IDs    IDGenerator
	Clock  Clock
	Log    Logf
}

// TxManager 别名，避免 app 包直接依赖 platform/tx 的具体类型。
type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// Handle 的流程刻意写成三段，每一段都对应一个 DDD 决策：
//
//	① 跨 BC 调用（库存预留）——「在自己的事务之外」，因为跨 BC 没有共享事务
//	② 本地事务 ——「订单落库 + 事件写 outbox」原子完成
//	③ 补偿 —— ② 失败时把 ① 退回去（Saga）
//
// 面试常问：「为什么不把 ①②③ 放一个分布式事务里？」
// 答：2PC 同步阻塞 + 协调者单点 + 跨网络持锁，高并发下吞吐塌陷；
// 互联网系统普遍选「最终一致 + 补偿」。这段代码就是那个选择的样子。
func (h *PlaceOrderHandler) Handle(ctx context.Context, cmd PlaceOrderCmd) (domain.OrderID, error) {
	// ---- 组装领域对象：校验规则由值对象/实体的构造函数负责 ----
	items := make([]domain.OrderItem, 0, len(cmd.Lines))
	stockLines := make([]StockLine, 0, len(cmd.Lines))
	skuIDs := make([]string, 0, len(cmd.Lines))
	for _, l := range cmd.Lines {
		price, err := domain.NewMoney(l.PriceCents, "CNY")
		if err != nil {
			return "", err
		}
		it, err := domain.NewOrderItem(l.SkuID, l.Name, l.Qty, price)
		if err != nil {
			return "", err
		}
		items = append(items, it)
		stockLines = append(stockLines, StockLine{SkuID: l.SkuID, Qty: l.Qty})
		skuIDs = append(skuIDs, l.SkuID)
	}

	id := h.IDs.NextOrderID()

	// ---- ① 跨 BC：预留库存。注意在本地事务之外 ----
	if err := h.Inv.Reserve(ctx, string(id), stockLines); err != nil {
		return "", err // 库存不足 => 订单根本不会被创建
	}

	// ---- ② 本地事务：订单落库 + 事件写 outbox，原子 ----
	err := h.Tx.Do(ctx, func(ctx context.Context) error {
		o, err := domain.NewOrder(id, cmd.BuyerID, items, h.Clock.Now())
		if err != nil {
			return err
		}
		if err := h.Repo.Save(ctx, o); err != nil {
			return err
		}
		return h.Events.AppendAll(ctx, o.PullEvents())
	})

	// ---- ③ Saga 补偿：订单没落库，占着的库存必须还回去 ----
	if err != nil {
		h.Log.log("        [订单] 落库失败(%v)，补偿释放已预留库存", err)
		if rerr := h.Inv.Release(ctx, string(id), skuIDs); rerr != nil {
			h.Log.log("        [订单] 补偿失败: %v（真实系统需落补偿失败表 + 告警）", rerr)
		}
		return "", err
	}
	return id, nil
}
