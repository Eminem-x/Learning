// Package outbox 实现「本地消息表 / Outbox 模式」。
//
// 要解决的问题（面试必考）：
//
//	「更新数据库」和「发 MQ 消息」是两个系统，双写无法原子。
//	先写库后发消息 => 消息可能丢；先发消息后写库 => 库可能失败但消息已出去。
//
// 解法：把「发消息的意图」也写进**同一个本地事务**。
//
//	BEGIN;
//	  UPDATE orders SET status='CANCELLED' ...;
//	  INSERT INTO outbox_events(...);      <-- 同一个事务
//	COMMIT;
//	-- 独立 relay 进程轮询 outbox（或 CDC 读 binlog）投递 MQ，投递成功再标记
//
// 代价：投递是「至少一次」，可能重复 => **消费端必须幂等**。
package outbox

import (
	"context"
	"fmt"
	"sync"

	"dddshop/internal/platform/eventbus"
	"dddshop/internal/platform/tx"
)

type Record struct {
	Envelope  eventbus.Envelope
	Delivered bool
	Attempts  int
}

// Store 就是那张 outbox 表。
type Store struct {
	txm *tx.Memory

	mu   sync.Mutex
	seq  int
	rows []*Record
}

func NewStore(txm *tx.Memory) *Store { return &Store{txm: txm} }

// Append 把事件登记进 outbox。
// 关键：如果当前处于事务中，写入会被 staged 到同一个工作单元 ——
// 业务数据和事件要么一起落，要么一起不落。
func (s *Store) Append(ctx context.Context, e eventbus.Envelope) error {
	s.mu.Lock()
	s.seq++
	e.ID = fmt.Sprintf("evt-%03d", s.seq)
	s.mu.Unlock()

	rec := &Record{Envelope: e}
	apply := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.rows = append(s.rows, rec)
	}

	if uow := s.txm.UoW(ctx); uow != nil {
		uow.Stage(nil, apply) // ← 与订单的写入进同一个工作单元
		return nil
	}
	apply()
	return nil
}

func (s *Store) Pending() []*Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Record
	for _, r := range s.rows {
		if !r.Delivered {
			out = append(out, r)
		}
	}
	return out
}

func (s *Store) Count() (total, pending int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rows {
		total++
		if !r.Delivered {
			pending++
		}
	}
	return
}

// Relay 消息中继：把 outbox 里未投递的事件推到事件总线。
// 真实项目里它是一个独立进程/协程，或者干脆用 Debezium 读 binlog。
type Relay struct {
	store *Store
	bus   *eventbus.Bus
	Logf  func(format string, args ...any)
}

func NewRelay(s *Store, b *eventbus.Bus) *Relay {
	return &Relay{store: s, bus: b, Logf: func(string, ...any) {}}
}

// RunOnce 跑一轮投递。
//
// duplicate=true 时**故意重复投递一次**，用来验证下游消费者是否幂等 ——
// 这不是 bug，这正是「至少一次」语义下真实会发生的事。
func (r *Relay) RunOnce(ctx context.Context, duplicate bool) error {
	for _, rec := range r.store.Pending() {
		times := 1
		if duplicate {
			times = 2
		}
		for i := 0; i < times; i++ {
			rec.Attempts++
			r.Logf("        [relay] 投递 %s (%s) 第 %d 次", rec.Envelope.ID, rec.Envelope.Name, rec.Attempts)
			if err := r.bus.Publish(ctx, rec.Envelope); err != nil {
				// 投递失败：不标记 Delivered，下一轮重试 —— 这就是「至少一次」
				r.Logf("        [relay] 投递失败，保留待重试: %v", err)
				return err
			}
		}
		rec.Delivered = true
	}
	return nil
}
