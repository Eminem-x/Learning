package infra

import (
	"context"
	"encoding/json"

	orderapp "dddshop/internal/order/app"
	"dddshop/internal/order/domain"
	"dddshop/internal/platform/eventbus"
	"dddshop/internal/platform/outbox"
)

// OutboxPublisher 实现 app.EventPublisher。
//
// 它做的事正是「**领域事件 → 集成事件**」的翻译：
//   - 输入：domain.DomainEvent（BC 内部概念，带领域语义）
//   - 输出：eventbus.Envelope（跨 BC 的扁平 JSON 契约，可版本化）
//
// 这个翻译不能省。直接把领域对象丢进 MQ，等于把内部模型变成对外 API，
// 以后每次重构领域模型都会把下游全部打挂。
type OutboxPublisher struct {
	store *outbox.Store
}

func NewOutboxPublisher(s *outbox.Store) *OutboxPublisher { return &OutboxPublisher{store: s} }

var _ orderapp.EventPublisher = (*OutboxPublisher)(nil)

func (p *OutboxPublisher) AppendAll(ctx context.Context, events []domain.DomainEvent) error {
	for _, e := range events {
		payload, err := json.Marshal(e)
		if err != nil {
			return err
		}
		env := eventbus.Envelope{
			Name:        e.EventName(),
			AggregateID: e.AggregateID(),
			OccurredAt:  e.OccurredAt(),
			Payload:     payload,
		}
		// ★ 关键：Append 会把写入 staged 到**当前事务**的工作单元，
		//   于是「订单状态变更」和「事件待发」同生共死。
		if err := p.store.Append(ctx, env); err != nil {
			return err
		}
	}
	return nil
}
