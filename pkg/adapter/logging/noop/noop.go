// Package noop 提供完全静默的日志实现。
//
// Noop 是一种具体策略而不是能力类型，因此放在 Adapter 层；这样 types 只描述
// Logger 契约，不会把“默认选择哪个实现”的决定混入公共类型目录。
package noop

import (
	"context"

	"github.com/rin721/micro-go/types/capability/logging"
)

// Logger 丢弃全部日志，可用于不关心输出的测试或显式关闭日志的场景。
type Logger struct{}

// New 创建无状态 Noop Logger。
func New() *Logger { return &Logger{} }

// Debug 丢弃 Debug 日志。
func (*Logger) Debug(context.Context, string, ...logging.Field) {}

// Info 丢弃 Info 日志。
func (*Logger) Info(context.Context, string, ...logging.Field) {}

// Warn 丢弃 Warn 日志。
func (*Logger) Warn(context.Context, string, ...logging.Field) {}

// Error 丢弃 Error 日志。
func (*Logger) Error(context.Context, string, ...logging.Field) {}

// With 返回同一无状态实例；Noop 没有需要复制的字段或资源。
func (l *Logger) With(...logging.Field) logging.Logger { return l }

// Named 返回同一无状态实例；命名空间对静默实现没有可观察影响。
func (l *Logger) Named(string) logging.Logger { return l }

// 编译期断言确保 Adapter 与项目 Logger 契约同步。
var _ logging.Logger = (*Logger)(nil)
