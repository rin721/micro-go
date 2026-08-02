// 本文件把 Kernel Slog 基线桥接为业务 Logger Capability，并实现受控配置 Reload。
package bootstrap

import (
	"context"
	"errors"

	kernelslog "github.com/rin721/micro-go/internal/adapter/kernel/logging/slog"
	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/module"
	"github.com/rin721/micro-go/internal/kernel/reload"
	"github.com/rin721/micro-go/types/capability/logging"
)

// loggingConfig 是组合根拥有的日志强类型配置，不向业务契约泄漏 Adapter 类型。
type loggingConfig struct {
	// Level 选择可原地更新的最小日志级别。
	Level string `yaml:"level" json:"level" validate:"required,oneof=debug info warn error"`
	// Output 选择标准流或由 Kernel Logger 打开的文件路径。
	Output string `yaml:"output" json:"output" validate:"required"`
	// JSON 决定 Handler 编码；变化时需要重启重建资源。
	JSON bool `yaml:"json" json:"json"`
}

// managedLogger 在组合根中桥接 Kernel Slog 配置和 Reload，但不取得 Logger 的关闭所有权。
type managedLogger struct {
	// Logger 是 Bootstrap 创建并最终关闭的同一个 Kernel 基线实例。
	*kernelslog.Logger
}

// loggingModule 保存待配置的 Kernel Logger，并声明其业务接口导出。
type loggingModule struct {
	// logger 由 Bootstrap 在配置加载前创建，Module 不拥有关闭责任。
	logger *kernelslog.Logger
}

// newLoggingModule 把唯一 Kernel Logger 引用装入日志模块值。
func newLoggingModule(logger *kernelslog.Logger) loggingModule { return loggingModule{logger: logger} }

// provide 把项目配置翻译为 Slog 配置，并返回不转移所有权的生命周期桥接对象。
func (m loggingModule) provide(value loggingConfig) (*managedLogger, error) {
	// nil Logger 表示组合根装配错误，不能创建第二套隐式实现作为回退。
	if m.logger == nil {
		return nil, errors.New("kernel logger is nil")
	}
	// Configure 可能打开文件；失败时基线 Logger 自行保留早期 Writer。
	if err := m.logger.Configure(kernelslog.Config{Level: value.Level, Output: value.Output, JSON: value.JSON}); err != nil {
		return nil, err
	}
	// managedLogger 嵌入同一指针，让业务 Binding 与 Kernel Manager 最终引用同一对象。
	return &managedLogger{Logger: m.logger}, nil
}

// Reload 把 Kernel Snapshot 翻译为 Slog 自有配置；两侧类型不会互相穿透包边界。
func (l *managedLogger) Reload(_ context.Context, snapshot config.Snapshot) (reload.Result, error) {
	// 从完整 Snapshot 读取本模块准确配置类型，缺失或解码错误保留原因链。
	value, err := config.Value[loggingConfig](snapshot)
	if err != nil {
		return reload.Ignored, err
	}
	// Adapter Apply 决定能否只更新 Level，组合根只翻译结果枚举。
	result, err := l.Apply(kernelslog.Config{Level: value.Level, Output: value.Output, JSON: value.JSON})
	if err != nil {
		return reload.Ignored, err
	}
	// 输出或编码变化映射为 Kernel RestartRequired，由 Runtime 统一退出。
	if result == kernelslog.ChangeRestartRequired {
		return reload.RestartRequired, nil
	}
	// 其余成功结果表示候选已经原地应用。
	return reload.Applied, nil
}

// Name 返回 Kernel 默认日志模块的稳定名称。
func (loggingModule) Name() string { return "kernel.logging.slog" }

// Register 声明日志配置、受管 Provider、Capability Binding 和跨模块 Export。
func (m loggingModule) Register(registry module.Registry) error {
	// logging 路径由本模块独占。
	if err := module.Config[loggingConfig](registry, "logging"); err != nil {
		return err
	}
	// 方法值 Provider 捕获同一 Kernel Logger 引用。
	if err := module.Provide(registry, m.provide); err != nil {
		return err
	}
	// 业务组件只依赖项目 Logger 接口，不接触 Slog 具体类型。
	if err := module.Bind[logging.Logger, *managedLogger](registry); err != nil {
		return err
	}
	// 导出接口供 applicationModule 等消费者使用。
	return module.Export[logging.Logger](registry)
}

// 编译期断言保证 managedLogger 满足业务日志契约。
var _ logging.Logger = (*managedLogger)(nil)

// 编译期断言保证 managedLogger 参与配置 Reload。
var _ reload.Reloader = (*managedLogger)(nil)
