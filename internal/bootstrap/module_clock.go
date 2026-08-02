package bootstrap

import (
	"github.com/rin721/micro-go/internal/kernel/module"
	clocksystem "github.com/rin721/micro-go/pkg/adapter/clock/system"
	"github.com/rin721/micro-go/types/capability/clock"
)

// clockModule 选择 System Clock，并将具体实现作为 Clock Capability 导出。
type clockModule struct{}

func (clockModule) Name() string { return "foundation.clock.system" }

func (clockModule) Register(registry module.Registry) error {
	if err := module.Provide(registry, clocksystem.New); err != nil {
		return err
	}
	if err := module.Bind[clock.Clock, *clocksystem.Clock](registry); err != nil {
		return err
	}
	return module.Export[clock.Clock](registry)
}
