package infra

import (
	"fmt"
	"sync"
	"time"

	orderapp "dddshop/internal/order/app"
	"dddshop/internal/order/domain"
)

// SeqIDGen 顺序 ID 生成器（demo 用；真实项目是雪花算法/号段/UUID）。
type SeqIDGen struct {
	mu sync.Mutex
	n  int
}

var _ orderapp.IDGenerator = (*SeqIDGen)(nil)

func (g *SeqIDGen) NextOrderID() domain.OrderID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return domain.OrderID(fmt.Sprintf("ORD-%03d", g.n))
}

// SystemClock 真实时钟。测试时换成 FixedClock，领域逻辑就完全可断言。
type SystemClock struct{}

var _ orderapp.Clock = SystemClock{}

func (SystemClock) Now() time.Time { return time.Now() }

// FixedClock 固定时钟，测试专用。
type FixedClock struct{ T time.Time }

func (c FixedClock) Now() time.Time { return c.T }
