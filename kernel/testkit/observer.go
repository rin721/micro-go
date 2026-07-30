// Package testkit 提供不参与生产运行的框架测试辅助能力。
package testkit

import (
	"sync"

	"github.com/rin721/micro-go/kernel/app"
	"github.com/rin721/micro-go/kernel/module"
)

type RecorderObserver struct {
	mu     sync.Mutex
	events []app.Event
}

func (r *RecorderObserver) Observe(event app.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *RecorderObserver) Events() []app.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]app.Event(nil), r.events...)
}

func (r *RecorderObserver) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = nil
}

func CompileModules(modules ...module.Module) (*app.Plan, error) {
	return app.Compile(app.WithModules(modules...))
}
