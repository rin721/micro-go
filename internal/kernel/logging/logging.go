// Package logging 定义 Kernel 必须持有且可显式替换的日志管理契约。
package logging

import capabilitylogging "github.com/rin721/micro-go/types/capability/logging"

// Manager 在 Kernel 基线 Logger 与组合根显式选择的增强 Logger 之间切换。
// Replace 不转移替换实例的资源所有权；Restore 只恢复基线，不关闭当前实例。
type Manager interface {
	// Logger 让 Runtime 始终可以通过同一个 Manager 写入当前有效实现。
	capabilitylogging.Logger
	// Replace 原子切换到已由组合根构造并持有的增强 Logger。
	Replace(capabilitylogging.Logger) error
	// Restore 恢复启动时的基线 Logger，但不关闭被替换对象。
	Restore()
}
