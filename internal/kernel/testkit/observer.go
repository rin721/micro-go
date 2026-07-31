// Package testkit 提供不参与生产运行的 Kernel 测试辅助能力。
package testkit

import (
	"sync"

	"github.com/rin721/micro-go/internal/kernel/app"
)

// RecorderObserver 线程安全地记录 Observer 事件，便于测试并发 Runner 和状态转换时
// 不依赖真实日志或指标系统。
type RecorderObserver struct {
	mu     sync.Mutex
	events []app.Event
}

// Observe 记录事件；互斥锁保护来自不同 goroutine 的调用。
func (r *RecorderObserver) Observe(event app.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

// Events 返回切片副本，测试修改返回值不会影响记录器内部状态。
func (r *RecorderObserver) Events() []app.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]app.Event(nil), r.events...)
}

// Reset 清空已有事件，适合在表驱动测试的不同阶段复用记录器。
func (r *RecorderObserver) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = nil
}
