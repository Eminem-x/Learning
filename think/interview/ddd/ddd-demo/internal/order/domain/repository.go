package domain

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("订单不存在")
	// ErrConflict 乐观锁冲突：加载后到保存前，别人改过这个聚合。
	ErrConflict = errors.New("并发修改冲突，请重试")
)

// Repository 是「端口（Port）」：**接口定义在领域层，实现在基础设施层**。
// 这就是依赖倒置 —— 领域层不认识 MySQL，MySQL 来适配领域层。
//
// 它和 DAO 的三个本质区别：
//  1. 面向**聚合根**，不面向表。一个聚合一个 Repository，
//     绝不给 OrderItem 单独建 Repository（那会绕过聚合根破坏不变性）。
//  2. 心智模型是「一个能持久化的内存集合」：FindByID / Save，仅此而已。
//  3. **不放复杂查询**。`FindByStatusAndTimeRangeOrderByX` 这种是读场景，
//     该走 CQRS 查询侧（见 app/query.go），塞进来就退化成 DAO 了。
type Repository interface {
	FindByID(ctx context.Context, id OrderID) (*Order, error)
	// Save 整个聚合一起存，内部做乐观锁版本比对。
	Save(ctx context.Context, o *Order) error
}
