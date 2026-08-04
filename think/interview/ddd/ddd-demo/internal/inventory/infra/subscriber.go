package infra

import (
	"context"
	"encoding/json"

	invapp "dddshop/internal/inventory/app"
	"dddshop/internal/platform/eventbus"
)

// orderCancelledPayload 是库存 BC **自己定义**的、对订单事件的解读结构。
//
// ★ 注意：这里刻意没有 import 订单 BC 的 domain 包。
// 库存 BC 只认「一份 JSON 契约」，不认订单的领域对象。
// 这样订单 BC 重构内部模型时，库存 BC 编译不会挂 —— 解耦才是真的。
// 这本质上也是一层轻量的防腐层（ACL）。
type orderCancelledPayload struct {
	OrderID string   `json:"order_id"`
	Reason  string   `json:"reason"`
	SkuIDs  []string `json:"sku_ids"`
}

// SubscribeOrderCancelled 把「订单已取消」翻译成库存 BC 的动作：释放预留。
//
// 这就是**跨聚合、跨 BC 的最终一致**：
// 订单取消的事务里没有碰任何库存数据，库存是稍后通过事件追上来的。
// 中间那段时间库存是「多占着的」—— 业务上可接受，换来的是两个 BC 彻底解耦。
func SubscribeOrderCancelled(bus *eventbus.Bus, svc *invapp.Service) {
	bus.Subscribe("order.cancelled", func(ctx context.Context, e eventbus.Envelope) error {
		var p orderCancelledPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return err
		}
		// Release 是幂等的（见 domain.Stock.Release），
		// 所以 Outbox「至少一次」的重复投递在这里完全无害。
		return svc.Release(ctx, p.OrderID, p.SkuIDs)
	})
}
