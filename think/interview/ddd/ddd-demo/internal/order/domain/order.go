// Package domain 是订单限界上下文的**领域层**。
//
// 铁律：这个包不 import 任何数据库、HTTP、MQ、ORM、第三方 SDK。
// 只有标准库。这意味着它可以脱离一切基础设施单测（见 order_test.go）。
package domain

import (
	"errors"
	"time"
)

type OrderID string

type Status uint8

const (
	StatusCreated   Status = iota + 1 // 待支付
	StatusPaid                        // 已支付
	StatusShipped                     // 已发货
	StatusCancelled                   // 已取消
)

func (s Status) String() string {
	switch s {
	case StatusCreated:
		return "待支付"
	case StatusPaid:
		return "已支付"
	case StatusShipped:
		return "已发货"
	case StatusCancelled:
		return "已取消"
	}
	return "未知"
}

var (
	ErrInvalidStatus = errors.New("当前状态不允许该操作")
	ErrEmptyOrder    = errors.New("订单至少要有一件商品")
	ErrInvalidQty    = errors.New("数量必须大于 0")
)

// OrderItem 是**聚合内实体**：它有业务身份（skuID），但生命周期完全依附于 Order。
// 外部拿不到它的指针，也没有 OrderItemRepository —— 想改明细只能通过聚合根。
type OrderItem struct {
	skuID string
	name  string // ★ 下单时的商品名「快照」：商品中心后来改名，历史订单不受影响
	qty   int
	price Money // ★ 成交价快照，同理
}

func NewOrderItem(skuID, name string, qty int, price Money) (OrderItem, error) {
	if skuID == "" || name == "" {
		return OrderItem{}, errors.New("skuID / 商品名不能为空")
	}
	if qty <= 0 {
		return OrderItem{}, ErrInvalidQty
	}
	return OrderItem{skuID: skuID, name: name, qty: qty, price: price}, nil
}

func (i OrderItem) SkuID() string   { return i.skuID }
func (i OrderItem) Name() string    { return i.name }
func (i OrderItem) Qty() int        { return i.qty }
func (i OrderItem) Price() Money    { return i.price }
func (i OrderItem) Subtotal() Money { return i.price.MulInt(i.qty) }

// Order 是**聚合根（Aggregate Root）**，本 demo 的核心。
//
// 它同时是两个边界：
//   - 一致性边界：`total == Σ items.Subtotal()` 这条不变性，任何时刻都成立
//   - 事务边界：一个事务只修改一个 Order 实例
//
// 三个设计细节值得注意：
//  1. 所有字段**不导出** —— 外部无法 `o.status = StatusShipped` 绕过规则
//  2. buyerID 是 string 不是 *User —— **聚合之间只用 ID 引用**（Vernon 原则 3）
//  3. events 在聚合内累积，由应用层统一取走发布
type Order struct {
	id      OrderID
	buyerID string
	items   []OrderItem
	status  Status
	total   Money
	version int // 乐观锁版本号
	created time.Time

	events []DomainEvent // 本次操作产生的领域事件（不落库，Save 后由应用层取走）
}

// NewOrder 是**工厂**：创建的一刻就保证不变性成立，不存在「半成品订单」。
func NewOrder(id OrderID, buyerID string, items []OrderItem, now time.Time) (*Order, error) {
	if id == "" || buyerID == "" {
		return nil, errors.New("订单号 / 买家不能为空")
	}
	total, err := sumItems(items)
	if err != nil {
		return nil, err
	}
	o := &Order{
		id: id, buyerID: buyerID, items: items,
		status: StatusCreated, total: total, version: 0, created: now,
	}
	o.raise(OrderPlaced{
		OrderID: string(id), BuyerID: buyerID,
		TotalCents: total.Cents(), Currency: total.Currency(), At: now,
	})
	return o, nil
}

// ---------------------------------------------------------------------------
// 业务行为：这些方法就是「状态机」。规则住在这里，不在 Service 里。
// ---------------------------------------------------------------------------

func (o *Order) Pay(now time.Time) error {
	if o.status != StatusCreated {
		return ErrInvalidStatus // 不变性：只有待支付订单能被支付（重复支付、已取消订单支付全被挡）
	}
	o.status = StatusPaid
	o.raise(OrderPaid{OrderID: string(o.id), TotalCents: o.total.Cents(), At: now})
	return nil
}

func (o *Order) Ship(now time.Time) error {
	if o.status != StatusPaid {
		return ErrInvalidStatus // 不变性：没付钱不能发货
	}
	o.status = StatusShipped
	return nil
}

// Cancel 演示两个高频考点：状态机约束 + **幂等**。
//
// 幂等为什么必须做在领域层？因为上游（MQ、用户重复点击、超时重试）
// 一定会重复调用。把幂等表达成「重复取消 = 什么都不做且不报错」，
// 是业务语义，不是技术补丁。
func (o *Order) Cancel(reason string, now time.Time) error {
	if o.status == StatusCancelled {
		return nil // 幂等：已取消再取消，静默成功，且**不重复产生事件**
	}
	if o.status == StatusShipped {
		return ErrInvalidStatus // 不变性：已发货不可取消
	}
	o.status = StatusCancelled
	o.raise(OrderCancelled{
		OrderID: string(o.id), Reason: reason, SkuIDs: o.skuIDs(), At: now,
	})
	return nil
}

// AddItem 演示「不变性在边界内**立即**成立」：
// 改明细必须同步重算总额，而且要么都改要么都不改。
func (o *Order) AddItem(it OrderItem) error {
	if o.status != StatusCreated {
		return ErrInvalidStatus
	}
	next := append(append([]OrderItem{}, o.items...), it)
	total, err := sumItems(next) // 先算，算成功才提交（币种不一致会在这里被拦下）
	if err != nil {
		return err
	}
	o.items, o.total = next, total
	return nil
}

// ---------------------------------------------------------------------------
// 事件与只读访问
// ---------------------------------------------------------------------------

func (o *Order) raise(e DomainEvent) { o.events = append(o.events, e) }

// PullEvents 取出并清空事件，由应用层在事务内写进 outbox。
func (o *Order) PullEvents() []DomainEvent {
	es := o.events
	o.events = nil
	return es
}

func (o *Order) ID() OrderID        { return o.id }
func (o *Order) BuyerID() string    { return o.buyerID }
func (o *Order) Status() Status     { return o.status }
func (o *Order) Total() Money       { return o.total }
func (o *Order) Version() int       { return o.version }
func (o *Order) Items() []OrderItem { return append([]OrderItem(nil), o.items...) } // 返回副本，防外部篡改

func (o *Order) skuIDs() []string {
	ids := make([]string, 0, len(o.items))
	for _, it := range o.items {
		ids = append(ids, it.skuID)
	}
	return ids
}

func sumItems(items []OrderItem) (Money, error) {
	if len(items) == 0 {
		return Money{}, ErrEmptyOrder
	}
	total := items[0].Subtotal()
	for _, it := range items[1:] {
		var err error
		if total, err = total.Add(it.Subtotal()); err != nil {
			return Money{}, err
		}
	}
	return total, nil
}

// ---------------------------------------------------------------------------
// 持久化快照：Go 做 DDD 绕不开的一道坎
//
// 领域对象字段不导出 => ORM 的反射填不进来，也读不出去。
// 所以要显式提供「快照 <-> 聚合」的转换。Snapshot 就是持久化对象（PO），
// 它属于领域层的「对外序列化契约」，Repository 实现负责把它映射成表结构。
//
// 好处：领域模型和表结构彻底解耦，加字段/换存储不会污染领域层。
// 代价：多写一份映射代码。这就是 Go 没有 JPA 脏检查要付的税。
// ---------------------------------------------------------------------------

type ItemSnapshot struct {
	SkuID      string
	Name       string
	Qty        int
	PriceCents int64
	Currency   string
}

type Snapshot struct {
	ID       string
	BuyerID  string
	Status   uint8
	Items    []ItemSnapshot
	Cents    int64
	Currency string
	Version  int
	Created  time.Time
}

func (o *Order) Snapshot() Snapshot {
	items := make([]ItemSnapshot, 0, len(o.items))
	for _, it := range o.items {
		items = append(items, ItemSnapshot{
			SkuID: it.skuID, Name: it.name, Qty: it.qty,
			PriceCents: it.price.cents, Currency: it.price.currency,
		})
	}
	return Snapshot{
		ID: string(o.id), BuyerID: o.buyerID, Status: uint8(o.status),
		Items: items, Cents: o.total.cents, Currency: o.total.currency,
		Version: o.version, Created: o.created,
	}
}

// Rehydrate 从快照重建聚合。
// 注意：**不执行任何业务规则，也不产生事件** —— 这是「恢复历史状态」，
// 不是「发生了新的业务动作」。把二者混为一谈是常见错误。
func Rehydrate(s Snapshot) *Order {
	items := make([]OrderItem, 0, len(s.Items))
	for _, i := range s.Items {
		items = append(items, OrderItem{
			skuID: i.SkuID, name: i.Name, qty: i.Qty,
			price: Money{cents: i.PriceCents, currency: i.Currency},
		})
	}
	return &Order{
		id: OrderID(s.ID), buyerID: s.BuyerID, status: Status(s.Status),
		items: items, total: Money{cents: s.Cents, currency: s.Currency},
		version: s.Version, created: s.Created,
	}
}

// SetVersion 仅供 Repository 在持久化成功后回写版本号。
// （有 ORM 脏检查的语言不需要这个方法，Go 只能显式做。）
func (o *Order) SetVersion(v int) { o.version = v }
