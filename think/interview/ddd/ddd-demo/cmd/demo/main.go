// Command demo 把两个限界上下文装配起来，跑一遍完整业务流程。
//
// 运行：go run ./cmd/demo
//
// 这个文件本身就是「组合根（Composition Root）」：
// 整个程序里唯一知道「谁的实现是谁」的地方。
// 上面所有层都只认接口，只有这里做真正的连线。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	invapp "dddshop/internal/inventory/app"
	invinfra "dddshop/internal/inventory/infra"
	orderapp "dddshop/internal/order/app"
	orderdomain "dddshop/internal/order/domain"
	orderinfra "dddshop/internal/order/infra"
	"dddshop/internal/platform/eventbus"
	"dddshop/internal/platform/outbox"
	"dddshop/internal/platform/tx"
)

func main() {
	ctx := context.Background()

	// =======================================================================
	// 装配（组合根）
	// =======================================================================
	// ★ 两个 BC 各有自己的事务管理器 = 各有自己的数据库。
	//   这在代码层面表达了 DDD 的硬约束：跨 BC 不存在共享事务。
	orderTx := tx.NewMemory()
	invTx := tx.NewMemory()

	bus := eventbus.New()

	// ---- 库存 BC ----
	stockRepo := invinfra.NewStockRepo(invTx)
	invSvc := &invapp.Service{Tx: invTx, Repo: stockRepo, Logf: logf}

	// ---- 订单 BC ----
	orderRepo := orderinfra.NewOrderRepo(orderTx)
	outboxStore := outbox.NewStore(orderTx)
	publisher := orderinfra.NewOutboxPublisher(outboxStore)
	invACL := orderinfra.NewInventoryACL(invSvc) // ★ 防腐层：订单 BC 与库存 BC 的唯一接触点
	ids := &orderinfra.SeqIDGen{}
	clock := orderinfra.SystemClock{}
	queries := orderinfra.NewOrderQueryService(orderRepo)

	placeOrder := &orderapp.PlaceOrderHandler{
		Tx: orderTx, Repo: orderRepo, Events: publisher, Inv: invACL,
		IDs: ids, Clock: clock, Log: logf,
	}
	payOrder := &orderapp.PayOrderHandler{Tx: orderTx, Repo: orderRepo, Events: publisher, Clock: clock}
	cancelOrder := &orderapp.CancelOrderHandler{Tx: orderTx, Repo: orderRepo, Events: publisher, Clock: clock}
	shipOrder := &orderapp.ShipOrderHandler{Tx: orderTx, Repo: orderRepo, Clock: clock}

	// ---- 消息中继 + 订阅关系 ----
	relay := outbox.NewRelay(outboxStore, bus)
	relay.Logf = logf

	// 库存 BC 订阅「订单已取消」→ 释放预留（跨 BC 最终一致）
	invinfra.SubscribeOrderCancelled(bus, invSvc)

	// 再挂两个「下游」，演示解耦的价值：
	// 新增这两个消费者，订单 BC 的代码**一行都没改**。
	bus.Subscribe("order.placed", func(_ context.Context, e eventbus.Envelope) error {
		logf("        [通知服务] 收到 %s，给买家发下单成功短信", e.AggregateID)
		return nil
	})
	bus.Subscribe("order.paid", func(_ context.Context, e eventbus.Envelope) error {
		var p struct {
			TotalCents int64 `json:"total_cents"`
		}
		_ = json.Unmarshal(e.Payload, &p)
		logf("        [积分服务] 收到 %s 已支付，加 %d 积分", e.AggregateID, p.TotalCents/100)
		return nil
	})

	banner()

	// =======================================================================
	step(1, "初始化库存（库存 BC）")
	// =======================================================================
	stockRepo.Seed("SKU-IPHONE", 5)
	stockRepo.Seed("SKU-CASE", 10)
	printStock(ctx, invSvc, "SKU-IPHONE", "SKU-CASE")

	// =======================================================================
	step(2, "下单成功 —— 跨 BC 预留库存 + 本地事务落库 + 事件写 outbox")
	// =======================================================================
	id1, err := placeOrder.Handle(ctx, orderapp.PlaceOrderCmd{
		BuyerID: "buyer-A",
		Lines: []orderapp.PlaceLine{
			{SkuID: "SKU-IPHONE", Name: "iPhone 17", Qty: 2, PriceCents: 5999_00},
		},
	})
	must(err)
	say("下单成功: %s", id1)
	printOrder(ctx, orderRepo, id1)
	printStock(ctx, invSvc, "SKU-IPHONE")
	printOutbox(outboxStore)

	// =======================================================================
	step(3, "下单失败 —— 库存不足触发 Saga 补偿")
	// =======================================================================
	say("买家要 1 个壳 + 10 台 iPhone（只剩 3 台）")
	say("库存 BC 会先成功预留壳，再在 iPhone 上失败 → 必须把壳补偿释放回去")
	_, err = placeOrder.Handle(ctx, orderapp.PlaceOrderCmd{
		BuyerID: "buyer-A",
		Lines: []orderapp.PlaceLine{
			{SkuID: "SKU-CASE", Name: "手机壳", Qty: 1, PriceCents: 59_00},
			{SkuID: "SKU-IPHONE", Name: "iPhone 17", Qty: 10, PriceCents: 5999_00},
		},
	})
	if errors.Is(err, orderapp.ErrOutOfStock) {
		say("下单被拒绝: %v", err)
		say("★ 注意错误类型：库存 BC 抛的是 invdomain.ErrInsufficient，")
		say("  被 ACL 翻译成了订单 BC 自己的 ErrOutOfStock —— 这就是防腐层在干活")
	} else {
		say("预期外的结果: %v", err)
	}
	printStock(ctx, invSvc, "SKU-IPHONE", "SKU-CASE")
	say("★ 壳的可用量回到 10 —— 补偿生效，没有留下脏预留")

	// =======================================================================
	step(4, "支付 + 发货 + 尝试取消 —— 状态机规则由聚合守住")
	// =======================================================================
	must(payOrder.Handle(ctx, orderapp.PayOrderCmd{OrderID: id1}))
	say("支付成功")
	must(shipOrder.Handle(ctx, id1))
	say("发货成功")
	printOrder(ctx, orderRepo, id1)

	err = cancelOrder.Handle(ctx, orderapp.CancelOrderCmd{OrderID: id1, Reason: "买家反悔"})
	say("尝试取消已发货订单 → %v", err)
	say("★ 这条规则写在 domain.Order.Cancel 里，不在任何 Service 里。")
	say("  所以不管从 HTTP、定时任务还是客服后台调用，都绕不过去。")

	// =======================================================================
	step(5, "再下一单并取消 —— 领域事件 + Outbox + 至少一次投递 + 幂等消费")
	// =======================================================================
	id2, err := placeOrder.Handle(ctx, orderapp.PlaceOrderCmd{
		BuyerID: "buyer-B",
		Lines: []orderapp.PlaceLine{
			{SkuID: "SKU-IPHONE", Name: "iPhone 17", Qty: 1, PriceCents: 5999_00},
			{SkuID: "SKU-CASE", Name: "手机壳", Qty: 1, PriceCents: 59_00},
		},
	})
	must(err)
	say("下单成功: %s", id2)
	printStock(ctx, invSvc, "SKU-IPHONE", "SKU-CASE")

	must(cancelOrder.Handle(ctx, orderapp.CancelOrderCmd{OrderID: id2, Reason: "超时未支付"}))
	say("订单已取消（此刻库存还没动！取消事务里根本没碰库存）")
	printStock(ctx, invSvc, "SKU-IPHONE", "SKU-CASE")
	printOutbox(outboxStore)

	say("")
	say("现在启动 relay 投递 outbox —— 并且**故意每条投两次**，模拟「至少一次」：")
	must(relay.RunOnce(ctx, true))
	printStock(ctx, invSvc, "SKU-IPHONE", "SKU-CASE")
	say("★ %s 占的库存被还回来了（iPhone 2→3、壳 9→10；剩下 2 台仍被 %s 占着）", id2, id1)
	say("★ 第二次投递走进了「幂等跳过」，没有多还一次 —— 幂等由 Stock 聚合内部保证")
	say("★ 但请注意 [通知服务] 打印了两次：它**没做幂等**，真实系统里买家会收到两条短信。")
	say("  「至少一次投递」是 Outbox 的固有代价，每个消费者都必须自己扛幂等。")
	printOutbox(outboxStore)

	// =======================================================================
	step(6, "乐观锁 —— 两个请求基于同一版本并发修改")
	// =======================================================================
	id3, err := placeOrder.Handle(ctx, orderapp.PlaceOrderCmd{
		BuyerID: "buyer-C",
		Lines:   []orderapp.PlaceLine{{SkuID: "SKU-CASE", Name: "手机壳", Qty: 1, PriceCents: 59_00}},
	})
	must(err)

	// 模拟两个并发请求：都在版本 v0 时把订单读了出来
	reqA, err := orderRepo.FindByID(ctx, id3)
	must(err)
	reqB, err := orderRepo.FindByID(ctx, id3)
	must(err)
	staleVersion := reqA.Version()
	say("请求 A 和请求 B 都读到了 %s（version=%d）", id3, staleVersion)

	errA := orderTx.Do(ctx, func(ctx context.Context) error {
		if err := reqA.Cancel("A 取消", clock.Now()); err != nil {
			return err
		}
		if err := orderRepo.Save(ctx, reqA); err != nil {
			return err
		}
		return publisher.AppendAll(ctx, reqA.PullEvents())
	})
	say("请求 A 提交 → %v（version 变成 %d）", errA, reqA.Version())

	errB := orderTx.Do(ctx, func(ctx context.Context) error {
		if err := reqB.Cancel("B 取消", clock.Now()); err != nil {
			return err
		}
		if err := orderRepo.Save(ctx, reqB); err != nil {
			return err
		}
		return publisher.AppendAll(ctx, reqB.PullEvents())
	})
	say("请求 B 提交 → %v", errB)
	say("★ B 基于过期数据（v%d）写入，被版本比对挡下。上层收到冲突后应「重新加载 → 重试」。", staleVersion)
	say("  这也是为什么聚合要设计得小：聚合越大，无关操作之间的冲突越多。")
	say("★ 顺带看一眼原子性：B 的事务里 Save 和 AppendAll 都 staged 了，")
	say("  提交时 Save 的版本校验没过 → **两个写入一起被丢弃**，outbox 不会留下幽灵事件。")

	// =======================================================================
	step(7, "CQRS 读侧 —— 查询不走聚合")
	// =======================================================================
	for _, buyer := range []string{"buyer-A", "buyer-B", "buyer-C"} {
		list, err := queries.ListByBuyer(ctx, buyer)
		must(err)
		for _, s := range list {
			say("%-8s %-8s %-6s %-14s %d 件", s.BuyerID, s.OrderID, s.Status, s.Total, s.ItemCount)
		}
	}
	say("★ 这条查询直接读存储拼 DTO，没有加载任何聚合、没有开事务。")
	say("  用聚合做列表查询是 DDD 项目性能崩掉的头号原因。")

	// =======================================================================
	step(8, "收尾：投递剩余事件（ORD-004 的取消事件也会让库存最终追上来）")
	// =======================================================================
	must(relay.RunOnce(ctx, false))
	printOutbox(outboxStore)
	printStock(ctx, invSvc, "SKU-IPHONE", "SKU-CASE")
	say("★ 最终态：iPhone 只剩 %s 占的 2 台，壳全部归还。", id1)
	say("  从「订单取消」到「库存归还」中间隔了几步异步 —— 这段时间数据是不一致的，")
	say("  这不是 bug，这就是最终一致性：用短暂的不一致换取两个 BC 的彻底解耦。")

	fmt.Println(`
===========================================================================
跑完了。建议按这个顺序读代码：

  1. internal/anemic/order_service.go        ← 先看「我们平时怎么写的」
  2. internal/order/domain/order.go          ← 再看聚合根：规则住在这里
  3. internal/order/domain/order_test.go     ← 领域层零依赖单测（毫秒级）
  4. internal/order/app/commands.go          ← 应用层有多薄
  5. internal/order/infra/inventory_acl.go   ← 防腐层
  6. internal/platform/outbox/outbox.go      ← 双写原子性的标准解法
  7. internal/inventory/domain/stock.go      ← 不超卖 + 幂等如何用模型表达

细节看 README.md。
===========================================================================`)
}

// ---------------------------------------------------------------------------
// 打印辅助
// ---------------------------------------------------------------------------

func banner() {
	fmt.Println(`
===========================================================================
 DDD Demo：订单 BC + 库存 BC
---------------------------------------------------------------------------
   [订单 BC]                              [库存 BC]
   api → app → domain ← infra             app → domain ← infra
                          │                              │
                          └── ACL 防腐层 ────────────────┘
                          └── Outbox → EventBus → 订阅者（最终一致）

   两个 BC 各有独立的事务管理器 = 各有独立的库，跨 BC 只有最终一致。
===========================================================================`)
}

func step(n int, title string) {
	fmt.Printf("\n%s\n步骤 %d：%s\n%s\n", strings.Repeat("─", 75), n, title, strings.Repeat("─", 75))
}

func say(format string, args ...any) {
	fmt.Printf("  "+format+"\n", args...)
}

func logf(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func printStock(ctx context.Context, svc *invapp.Service, skus ...string) {
	parts := make([]string, 0, len(skus))
	for _, sku := range skus {
		n, err := svc.Available(ctx, sku)
		if err != nil {
			parts = append(parts, fmt.Sprintf("%s=<%v>", sku, err))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s 可用=%d", sku, n))
	}
	say("库存: %s", strings.Join(parts, "  |  "))
}

func printOrder(ctx context.Context, repo *orderinfra.OrderRepo, id orderdomain.OrderID) {
	o, err := repo.FindByID(ctx, id)
	if err != nil {
		say("订单 %s: %v", id, err)
		return
	}
	say("订单 %s: 状态=%s 总额=%s 明细=%d 件 version=%d",
		o.ID(), o.Status(), o.Total(), len(o.Items()), o.Version())
}

func printOutbox(s *outbox.Store) {
	total, pending := s.Count()
	say("outbox: 共 %d 条，待投递 %d 条", total, pending)
}
