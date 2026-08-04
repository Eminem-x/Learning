// Package app 是订单 BC 的**应用层**。
//
// 应用层的唯一职责是**编排**：开事务、鉴权、调 Repository、调领域对象、发事件。
// 判断一个应用服务写得对不对，只有一条标准：
//
//	里面**不应该出现任何一个属于业务规则的 if**。
//
// 所有 `if 状态 == X 则不允许 Y` 都必须住在领域层。
package app

import (
	"context"
	"errors"
	"time"

	"dddshop/internal/order/domain"
)

var ErrOutOfStock = errors.New("库存不足，下单失败")

// StockLine 是订单 BC 用**自己的语言**描述的库存诉求。
// 它不是库存 BC 的类型 —— 两个 BC 之间的翻译由 ACL 负责（infra/inventory_acl.go）。
type StockLine struct {
	SkuID string
	Qty   int
}

// InventoryPort 出站端口：订单 BC 需要「有人帮我预留库存」。
//
// ★ 端口用**订单 BC 的词汇**定义，而不是照抄库存服务的接口。
// 这样将来库存换成第三方 WMS、或者从本地调用变成 gRPC，
// 只需要换一个适配器实现，应用层代码零改动。
type InventoryPort interface {
	Reserve(ctx context.Context, orderID string, lines []StockLine) error
	Release(ctx context.Context, orderID string, skuIDs []string) error
}

// EventPublisher 出站端口：把领域事件交出去（实现里会写进 outbox）。
type EventPublisher interface {
	AppendAll(ctx context.Context, events []domain.DomainEvent) error
}

// IDGenerator / Clock：把「不确定性」挡在领域层之外。
//
// 为什么时间要做成端口？因为 time.Now() 散落在领域层会让单测无法断言。
// 领域方法签名里的 now time.Time 就是这个思路的落地。
type IDGenerator interface {
	NextOrderID() domain.OrderID
}

type Clock interface {
	Now() time.Time
}

type Logf func(format string, args ...any)

func (l Logf) log(format string, args ...any) {
	if l != nil {
		l(format, args...)
	}
}
