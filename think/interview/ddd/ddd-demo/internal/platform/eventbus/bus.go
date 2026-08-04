// Package eventbus 是一个极简的进程内事件总线。
//
// 真实项目里跨 BC 的集成事件会走 Kafka/RocketMQ；
// 这里用内存总线，是为了让 demo 零依赖可跑，
// 但**语义完全一致**：发布方不知道谁在消费，消费方不知道谁发布的。
package eventbus

import (
	"context"
	"sync"
	"time"
)

// Envelope 是**集成事件**的信封：跨 BC 传输的扁平 DTO。
//
// 划重点：这里绝不能塞领域对象（*domain.Order）。
// 对外契约必须是可序列化、可版本化的扁平结构，
// 否则订单 BC 改一个私有字段，下游全部编译不过 —— 那就白解耦了。
type Envelope struct {
	ID          string    `json:"id"`           // 事件唯一 ID，消费端幂等去重用
	Name        string    `json:"name"`         // 如 "order.cancelled"
	AggregateID string    `json:"aggregate_id"` // 来源聚合 ID
	OccurredAt  time.Time `json:"occurred_at"`
	Payload     []byte    `json:"payload"` // JSON 编码的事件体
}

type Handler func(ctx context.Context, e Envelope) error

type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

func New() *Bus { return &Bus{handlers: make(map[string][]Handler)} }

func (b *Bus) Subscribe(eventName string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventName] = append(b.handlers[eventName], h)
}

// Publish 同步投递给所有订阅者。任一 handler 失败即返回错误，
// 由上层（Outbox relay）决定是否重投 —— 这就是「至少一次」语义的来源。
func (b *Bus) Publish(ctx context.Context, e Envelope) error {
	b.mu.RLock()
	hs := append([]Handler(nil), b.handlers[e.Name]...)
	b.mu.RUnlock()

	for _, h := range hs {
		if err := h(ctx, e); err != nil {
			return err
		}
	}
	return nil
}
