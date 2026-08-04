// Package anemic 是**反例**，只供对照阅读，demo 主流程不会用到它。
//
// 这里是同一个订单业务的「贫血模型 + 事务脚本」写法 ——
// 也就是我们绝大多数项目里真实长的样子。
//
// 建议的读法：先读这个文件，再读 internal/order/domain/order.go，
// 感受同一段业务在两种写法下的差别。差别不在「分了几层包」，
// 而在**业务规则住在哪里**。
package anemic

import (
	"errors"
	"time"
)

// ❌ 贫血模型：Order 只是一个数据袋（DTO），没有任何行为。
//
// 字段全部导出 => 系统里任何一个角落都可以 `o.Status = 4`，
// 绕过所有校验。规则守不住，因为根本没人守。
type Order struct {
	ID         string
	BuyerID    string
	Status     int     // 1待支付 2已支付 3已发货 4已取消 —— 魔法数字，靠注释维持
	TotalYuan  float64 // ❌ 浮点存金额：0.1+0.2 != 0.3，对账时哭都来不及
	ItemCount  int
	UpdateTime time.Time
}

type OrderDAO interface {
	Get(id string) (*Order, error)
	Update(o *Order) error
}

type StockAPI interface {
	Release(orderID string, skuIDs []string) error
}

type MQ interface {
	Send(topic string, body []byte) error
}

// OrderService 事务脚本：所有业务规则都在这里，用一堆 if 拼出来。
type OrderService struct {
	dao   OrderDAO
	stock StockAPI
	mq    MQ
}

// CancelOrder 看着挺正常，问题全在暗处。
func (s *OrderService) CancelOrder(orderID, reason string, skuIDs []string) error {
	o, err := s.dao.Get(orderID)
	if err != nil {
		return err
	}

	// 【问题 1】状态机规则散落在 Service 里。
	// 这条「已发货不可取消」在 CancelOrder 里写了，
	// 但 batchCancel / 定时任务超时取消 / 客服后台强制取消 里呢？
	// 靠人记得抄一遍。漏一个地方 = 线上事故。
	if o.Status == 3 {
		return errors.New("已发货，不能取消")
	}
	// 【问题 2】幂等靠 Service 自觉，不是模型语义。
	if o.Status == 4 {
		return nil
	}

	o.Status = 4 // 【问题 3】直接赋值，没有任何守卫。任何人在任何文件里都能这么干
	o.UpdateTime = time.Now()

	if err := s.dao.Update(o); err != nil { // 【问题 4】全字段 update，没有版本号 => 并发覆盖
		return err
	}

	// 【问题 5】跨 BC 调用夹在中间：
	// 库存释放失败了怎么办？订单已经取消了，库存还占着，两边不一致，没人兜底。
	if err := s.stock.Release(orderID, skuIDs); err != nil {
		return err // 直接返回 error，调用方看到失败，但订单其实已经取消了
	}

	// 【问题 6】双写非原子：
	// 上面 dao.Update 已经提交，这里 mq.Send 挂了 => 消息永久丢失，下游永远收不到。
	// 反过来先发消息再写库 => 库失败但消息已出去，下游按「已取消」处理了幽灵订单。
	if err := s.mq.Send("order.cancelled", []byte(`{"order_id":"`+orderID+`"}`)); err != nil {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// 对照表：同样的六个问题，DDD 版分别怎么解
//
//	问题 1 状态机散落  -> domain.Order.Cancel()：规则只有一份，绕不过去
//	问题 2 幂等靠自觉  -> Cancel 内部「已取消则静默返回且不发事件」，是模型语义
//	问题 3 裸赋值      -> 字段不导出，只能通过行为方法改状态
//	问题 4 并发覆盖    -> Snapshot.Version + Repository.Save 乐观锁
//	问题 5 跨 BC 一致  -> 事件驱动 + 幂等消费 + Saga 补偿
//	问题 6 双写非原子  -> Outbox：业务变更与事件写入同一个本地事务
//
// 注意最关键的一点：这六条**没有一条**是靠「多分几层包」解决的。
// 目录结构只是结果，不是手段。
// ---------------------------------------------------------------------------
