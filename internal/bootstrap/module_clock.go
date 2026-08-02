// 本文件在组合根选择系统时钟 Adapter，并公开项目 Clock Capability。
package bootstrap

import (
	"github.com/rin721/micro-go/internal/kernel/module"
	clocksystem "github.com/rin721/micro-go/pkg/adapter/clock/system"
	"github.com/rin721/micro-go/types/capability/clock"
)

// clockModule 选择 System Clock，并将具体实现作为 Clock Capability 导出。
type clockModule struct{}

// Name 返回依赖图中的稳定时钟模块名。
func (clockModule) Name() string { return "foundation.clock.system" }

// Register 声明构造、接口绑定和跨模块导出三项静态事实。
func (clockModule) Register(registry module.Registry) error {
	// Provider 创建无状态具体 Clock 实例。
	if err := module.Provide(registry, clocksystem.New); err != nil {
		return err
	}
	// Binding 让 Clock 接口与具体实例保持同一对象。
	if err := module.Bind[clock.Clock, *clocksystem.Clock](registry); err != nil {
		return err
	}
	// Export 是其他模块通过接口依赖该能力的显式授权。
	return module.Export[clock.Clock](registry)
}
