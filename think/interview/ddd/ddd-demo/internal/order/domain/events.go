package domain

import "time"

// 事件名一律用**过去式**：领域事件描述的是「已经发生的事实」，不是命令。
const (
	EventOrderPlaced    = "order.placed"
	EventOrderPaid      = "order.paid"
	EventOrderCancelled = "order.cancelled"
)

// DomainEvent 领域事件接口。
//
// 领域事件 vs 集成事件：
//   - 领域事件（这里）：BC 内部概念，由聚合产生，可以携带领域含义
//   - 集成事件（eventbus.Envelope）：跨 BC 的对外契约，必须扁平、可版本化
//
// 二者的翻译发生在基础设施层（infra/event_publisher.go）。
type DomainEvent interface {
	EventName() string
	AggregateID() string
	OccurredAt() time.Time
}

type OrderPlaced struct {
	OrderID    string    `json:"order_id"`
	BuyerID    string    `json:"buyer_id"`
	TotalCents int64     `json:"total_cents"`
	Currency   string    `json:"currency"`
	At         time.Time `json:"at"`
}

func (e OrderPlaced) EventName() string     { return EventOrderPlaced }
func (e OrderPlaced) AggregateID() string   { return e.OrderID }
func (e OrderPlaced) OccurredAt() time.Time { return e.At }

type OrderPaid struct {
	OrderID    string    `json:"order_id"`
	TotalCents int64     `json:"total_cents"`
	At         time.Time `json:"at"`
}

func (e OrderPaid) EventName() string     { return EventOrderPaid }
func (e OrderPaid) AggregateID() string   { return e.OrderID }
func (e OrderPaid) OccurredAt() time.Time { return e.At }

// OrderCancelled 携带 SkuIDs，是为了让下游（库存 BC）不必回调订单 BC 查明细。
// 事件应当「自包含」：消费者拿到事件就能干活，否则解耦是假的。
type OrderCancelled struct {
	OrderID string    `json:"order_id"`
	Reason  string    `json:"reason"`
	SkuIDs  []string  `json:"sku_ids"`
	At      time.Time `json:"at"`
}

func (e OrderCancelled) EventName() string     { return EventOrderCancelled }
func (e OrderCancelled) AggregateID() string   { return e.OrderID }
func (e OrderCancelled) OccurredAt() time.Time { return e.At }
