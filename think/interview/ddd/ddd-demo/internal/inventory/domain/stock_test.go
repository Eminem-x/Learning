package domain

import "testing"

// 核心不变性：任何路径都不能把可用量做成负数（不超卖）
func TestNeverOversell(t *testing.T) {
	s := NewStock("SKU-A", 5)

	if err := s.Reserve("ORD-1", 3); err != nil {
		t.Fatal(err)
	}
	if s.Available() != 2 {
		t.Fatalf("可用量应为 2, got %d", s.Available())
	}

	if err := s.Reserve("ORD-2", 3); err != ErrInsufficient {
		t.Fatalf("超量预留应被拒绝, got %v", err)
	}
	if s.Available() != 2 {
		t.Fatalf("失败的预留不应改变状态, got %d", s.Available())
	}

	// 边界：刚好取完是允许的
	if err := s.Reserve("ORD-3", 2); err != nil {
		t.Fatalf("刚好取完应允许, got %v", err)
	}
	if s.Available() != 0 {
		t.Fatalf("可用量应为 0, got %d", s.Available())
	}
}

// 幂等 1：同一订单重复预留只占一次坑
func TestReserveIsIdempotent(t *testing.T) {
	s := NewStock("SKU-A", 5)
	for i := 0; i < 3; i++ {
		if err := s.Reserve("ORD-1", 2); err != nil {
			t.Fatalf("第 %d 次预留失败: %v", i+1, err)
		}
	}
	if s.Available() != 3 {
		t.Fatalf("重复预留被多次计算, 可用量 got %d, want 3", s.Available())
	}
}

// 幂等 2：重复释放无副作用 —— 这是 Outbox「至少一次」投递的安全网
func TestReleaseIsIdempotent(t *testing.T) {
	s := NewStock("SKU-A", 5)
	if err := s.Reserve("ORD-1", 2); err != nil {
		t.Fatal(err)
	}

	if !s.Release("ORD-1") {
		t.Fatal("首次释放应返回 true")
	}
	if s.Available() != 5 {
		t.Fatalf("释放后应恢复到 5, got %d", s.Available())
	}

	if s.Release("ORD-1") {
		t.Fatal("重复释放应返回 false")
	}
	if s.Available() != 5 {
		t.Fatalf("重复释放不应多还库存, got %d", s.Available())
	}
	if s.Release("ORD-NOT-EXIST") {
		t.Fatal("释放不存在的预留应返回 false")
	}
}

func TestInvalidQty(t *testing.T) {
	s := NewStock("SKU-A", 5)
	if err := s.Reserve("ORD-1", 0); err != ErrInvalidQty {
		t.Fatalf("数量 0 应被拒绝, got %v", err)
	}
	if err := s.Reserve("ORD-1", -1); err != ErrInvalidQty {
		t.Fatalf("负数应被拒绝, got %v", err)
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	s := NewStock("SKU-A", 10)
	_ = s.Reserve("ORD-1", 2)
	_ = s.Reserve("ORD-2", 3)
	s.SetVersion(4)

	back := Rehydrate(s.Snapshot())
	if back.SkuID() != "SKU-A" || back.Available() != 5 || back.Version() != 4 {
		t.Fatalf("往返丢失信息: %+v", back.Snapshot())
	}
	if back.Release("ORD-1") != true || back.Available() != 7 {
		t.Fatal("重建后的聚合行为异常")
	}
}
