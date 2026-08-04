package app

import "context"

// ---------------------------------------------------------------------------
// CQRS 的读侧
//
// 为什么查询不走 Repository + 聚合？
//
//	写模型（聚合）是为「保护不变性」设计的：加载整个聚合、跑状态机、乐观锁。
//	读模型是为「查得快、查得省」设计的：只要几个字段，要分页、要排序、要联表。
//	用聚合做列表查询 = 为了显示 20 行订单，加载 20 个完整聚合外加所有明细。
//
// 所以读写分开建模。这是 CQRS 的第一档（同一个库、两套模型），
// 零额外基础设施成本，收益却最大 —— 也是最该先落地的一档。
//
// 第二档：读走从库 / Elasticsearch（你熟悉的「订单写 MySQL、查询走 ES」就是这个）。
// 第三档：事件溯源，成本极高，绝大多数系统不需要。
// ---------------------------------------------------------------------------

// OrderSummary 是**读模型 DTO**，直接面向 UI 的形状。
// 它跟 domain.Order 长得不一样，这是**特意**的，不是重复。
type OrderSummary struct {
	OrderID   string
	BuyerID   string
	Status    string
	Total     string
	ItemCount int
}

// OrderQuery 查询端口。实现里可以随便写 SQL / 查 ES，
// 不需要经过聚合，也不需要遵守「一个事务一个聚合」——读没有一致性边界问题。
type OrderQuery interface {
	ListByBuyer(ctx context.Context, buyerID string) ([]OrderSummary, error)
}
