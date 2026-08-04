package domain

// 这个测试文件是整个 demo 最该先读的东西之一。
//
// 注意它的 import：只有标准库。
// 不起 Docker、不连 MySQL、不 mock 任何框架，毫秒级跑完 ——
// 这就是「领域层零外部依赖」换来的收益，也是判断一个项目
// 是「真 DDD」还是「目录 DDD」最直接的标尺。

import (
	"testing"
	"time"
)

var now = time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)

func mustItem(t *testing.T, sku string, qty int, cents int64) OrderItem {
	t.Helper()
	it, err := NewOrderItem(sku, "商品-"+sku, qty, MustMoney(cents, "CNY"))
	if err != nil {
		t.Fatalf("构造明细失败: %v", err)
	}
	return it
}

func mustOrder(t *testing.T, items ...OrderItem) *Order {
	t.Helper()
	o, err := NewOrder("ORD-1", "buyer-1", items, now)
	if err != nil {
		t.Fatalf("建单失败: %v", err)
	}
	return o
}

// 不变性 1：总额恒等于明细之和
func TestTotalEqualsSumOfItems(t *testing.T) {
	o := mustOrder(t, mustItem(t, "SKU-A", 2, 599_00), mustItem(t, "SKU-B", 3, 10_00))
	want := MustMoney(2*599_00+3*10_00, "CNY")
	if !o.Total().Equals(want) {
		t.Fatalf("总额错误: got %s, want %s", o.Total(), want)
	}

	// 加一件后总额必须立刻同步 —— 不变性在聚合边界内「时刻成立」
	if err := o.AddItem(mustItem(t, "SKU-C", 1, 1_00)); err != nil {
		t.Fatal(err)
	}
	want2 := MustMoney(2*599_00+3*10_00+1_00, "CNY")
	if !o.Total().Equals(want2) {
		t.Fatalf("加明细后总额未同步: got %s, want %s", o.Total(), want2)
	}
}

// 不变性 2：空订单不允许存在
func TestEmptyOrderRejected(t *testing.T) {
	if _, err := NewOrder("ORD-X", "buyer-1", nil, now); err != ErrEmptyOrder {
		t.Fatalf("空订单应被拒绝, got %v", err)
	}
}

// 不变性 3：币种不一致必须被拦下（值对象自我保护）
func TestCurrencyMismatchRejected(t *testing.T) {
	cny := mustItem(t, "SKU-A", 1, 100_00)
	usd, _ := NewOrderItem("SKU-USD", "美元商品", 1, MustMoney(20_00, "USD"))
	if _, err := NewOrder("ORD-Y", "buyer-1", []OrderItem{cny, usd}, now); err != ErrCurrencyMismatch {
		t.Fatalf("混币种应被拒绝, got %v", err)
	}
}

// 状态机 1：已发货不可取消
func TestShippedOrderCannotBeCancelled(t *testing.T) {
	o := mustOrder(t, mustItem(t, "SKU-A", 1, 100_00))
	if err := o.Pay(now); err != nil {
		t.Fatal(err)
	}
	if err := o.Ship(now); err != nil {
		t.Fatal(err)
	}
	if err := o.Cancel("不想要了", now); err != ErrInvalidStatus {
		t.Fatalf("已发货订单应拒绝取消, got %v", err)
	}
	if o.Status() != StatusShipped {
		t.Fatalf("状态被非法修改: %v", o.Status())
	}
}

// 状态机 2：没付钱不能发货
func TestCannotShipUnpaidOrder(t *testing.T) {
	o := mustOrder(t, mustItem(t, "SKU-A", 1, 100_00))
	if err := o.Ship(now); err != ErrInvalidStatus {
		t.Fatalf("未支付订单应拒绝发货, got %v", err)
	}
}

// 状态机 3：重复支付被挡
func TestCannotPayTwice(t *testing.T) {
	o := mustOrder(t, mustItem(t, "SKU-A", 1, 100_00))
	if err := o.Pay(now); err != nil {
		t.Fatal(err)
	}
	if err := o.Pay(now); err != ErrInvalidStatus {
		t.Fatalf("重复支付应被拒绝, got %v", err)
	}
}

// 幂等：重复取消静默成功，且不重复产生事件
func TestCancelIsIdempotent(t *testing.T) {
	o := mustOrder(t, mustItem(t, "SKU-A", 1, 100_00))
	o.PullEvents() // 丢掉建单事件

	if err := o.Cancel("超时未支付", now); err != nil {
		t.Fatal(err)
	}
	first := o.PullEvents()
	if len(first) != 1 || first[0].EventName() != EventOrderCancelled {
		t.Fatalf("第一次取消应产生 1 个 OrderCancelled, got %+v", first)
	}

	if err := o.Cancel("再取消一次", now); err != nil {
		t.Fatalf("重复取消应静默成功, got %v", err)
	}
	if second := o.PullEvents(); len(second) != 0 {
		t.Fatalf("重复取消不应再产生事件, got %+v", second)
	}
}

// 事件：建单产生 OrderPlaced，且 PullEvents 后被清空
func TestPlaceOrderRaisesEventOnce(t *testing.T) {
	o := mustOrder(t, mustItem(t, "SKU-A", 1, 100_00))
	es := o.PullEvents()
	if len(es) != 1 || es[0].EventName() != EventOrderPlaced {
		t.Fatalf("应产生 1 个 OrderPlaced, got %+v", es)
	}
	if again := o.PullEvents(); len(again) != 0 {
		t.Fatalf("事件应已被取走清空, got %+v", again)
	}
}

// 封装：Items() 返回副本，外部拿到的切片改不动聚合内部状态
func TestItemsReturnsCopy(t *testing.T) {
	o := mustOrder(t, mustItem(t, "SKU-A", 1, 100_00))
	got := o.Items()
	got[0] = mustItem(t, "HACK", 999, 1)
	if o.Items()[0].SkuID() != "SKU-A" {
		t.Fatal("外部通过返回值篡改了聚合内部状态")
	}
}

// 持久化往返：Snapshot -> Rehydrate 必须完全还原，且不产生事件
func TestSnapshotRoundTrip(t *testing.T) {
	o := mustOrder(t, mustItem(t, "SKU-A", 2, 599_00))
	if err := o.Pay(now); err != nil {
		t.Fatal(err)
	}
	o.SetVersion(7)

	back := Rehydrate(o.Snapshot())
	if back.ID() != o.ID() || back.Status() != o.Status() ||
		!back.Total().Equals(o.Total()) || back.Version() != 7 {
		t.Fatalf("往返丢失信息: %+v", back.Snapshot())
	}
	if es := back.PullEvents(); len(es) != 0 {
		t.Fatalf("重建历史状态不应产生事件, got %+v", es)
	}
}
