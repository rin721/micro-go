// Package system 提供系统时钟适配器。
package system

import (
	"time"

	clockcontract "github.com/rin721/micro-go/capability/clock"
	"github.com/rin721/micro-go/kernel/module"
)

// Clock 使用操作系统墙上时钟实现 capability/clock.Clock。
type Clock struct{}

// New 创建无状态系统时钟。
func New() *Clock { return &Clock{} }

// Now 返回 time.Now 的当前结果。
func (*Clock) Now() time.Time { return time.Now() }

// Module 把系统时钟作为 clock.Clock 契约注册并导出。
type Module struct{}

// Name 返回稳定模块名，用于依赖图和错误定位。
func (Module) Name() string { return "clock-system" }

// Register 声明 Provider、Binding 和 Export；业务组件只依赖 clock.Clock。
func (Module) Register(registry module.Registry) error {
	if err := module.Provide(registry, New); err != nil {
		return err
	}
	if err := module.Bind[clockcontract.Clock, *Clock](registry); err != nil {
		return err
	}
	return module.Export[clockcontract.Clock](registry)
}

// 编译期断言确保 Adapter 始终满足项目契约，不需要在运行期做能力探测。
var _ clockcontract.Clock = (*Clock)(nil)
