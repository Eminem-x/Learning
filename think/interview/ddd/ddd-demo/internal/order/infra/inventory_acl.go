package infra

import (
	"context"
	"errors"

	invapp "dddshop/internal/inventory/app"
	invdomain "dddshop/internal/inventory/domain"
	orderapp "dddshop/internal/order/app"
)

// InventoryACL 是**防腐层（Anticorruption Layer）**——上下文映射里最常用、最该会写的一种。
//
// 它的职责只有一个：**翻译**。
//
//	订单 BC 的语言  <--->  库存 BC 的语言
//	orderapp.StockLine   invapp.Line
//	orderapp.ErrOutOfStock <- invdomain.ErrInsufficient / ErrSkuNotFound
//
// 为什么非要多这一层？
//  1. 订单 BC 的领域层/应用层**永远不会 import 库存 BC 的任何类型**。
//     库存服务改字段、改错误码、甚至整个换成第三方 WMS，只需要改这一个文件。
//  2. 外部的模型往往很烂（想想第三方支付回调里那个 trade_status="TRADE_SUCCESS"
//     和字符串类型的金额），不挡住它，烂模型会渗透到你的核心域里，
//     几个月后你的 Order 里就会长出一个叫 TradeStatus 的 string 字段。
//
// ★ ACL 是整个订单 BC 里**唯一**允许同时看见两个上下文的地方。
//
// 本 demo 里它是进程内直接调用；真实项目里这里通常是一次 gRPC/HTTP 调用
// —— 但**位置和职责完全一样**，换的只是传输方式。
type InventoryACL struct {
	svc *invapp.Service
}

func NewInventoryACL(svc *invapp.Service) *InventoryACL { return &InventoryACL{svc: svc} }

// 编译期断言：确保 ACL 实现了订单 BC 定义的端口。
var _ orderapp.InventoryPort = (*InventoryACL)(nil)

func (a *InventoryACL) Reserve(ctx context.Context, orderID string, lines []orderapp.StockLine) error {
	// 模型翻译：订单的词汇 -> 库存的词汇
	invLines := make([]invapp.Line, 0, len(lines))
	for _, l := range lines {
		invLines = append(invLines, invapp.Line{SkuID: l.SkuID, Qty: l.Qty})
	}

	err := a.svc.Reserve(ctx, orderID, invLines)

	// 错误翻译：库存的失败语义 -> 订单能理解的失败语义。
	// 订单 BC 不关心「是 SKU 不存在还是数量不够」，它只关心「下不了单」。
	switch {
	case errors.Is(err, invdomain.ErrInsufficient),
		errors.Is(err, invdomain.ErrSkuNotFound):
		return orderapp.ErrOutOfStock
	}
	return err
}

func (a *InventoryACL) Release(ctx context.Context, orderID string, skuIDs []string) error {
	return a.svc.Release(ctx, orderID, skuIDs)
}
