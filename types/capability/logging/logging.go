// Package logging 定义实现无关的结构化日志能力。
package logging

import (
	"context"
	"time"
)

// Field 是项目自有的结构化日志字段。Value 保留普通 Go 值，由具体 Adapter 转换为
// Zap Field 或 slog Attr，从而阻止第三方日志类型污染业务签名。
type Field struct {
	// Key 是结构化字段名称。
	Key string
	// Value 是交给具体 Adapter 编码的普通 Go 值。
	Value any
}

// String 创建字符串字段。
func String(key, value string) Field { return Field{Key: key, Value: value} }

// Int 创建整数值字段。
func Int(key string, value int) Field { return Field{Key: key, Value: value} }

// Bool 创建布尔值字段。
func Bool(key string, value bool) Field { return Field{Key: key, Value: value} }

// Duration 创建持续时间字段，并保留 time.Duration 的类型信息供 Adapter 编码。
func Duration(key string, value time.Duration) Field { return Field{Key: key, Value: value} }

// Time 创建时间字段。
func Time(key string, value time.Time) Field { return Field{Key: key, Value: value} }

// Error 使用统一的 error 键创建错误字段。
func Error(err error) Field { return Field{Key: "error", Value: err} }

// Logger 是业务代码可依赖的最小结构化日志契约。
// Context 允许具体实现读取调用链信息；With 和 Named 返回派生 Logger，但共享底层资源所有权。
type Logger interface {
	// Debug 记录仅用于排障的低级别事件。
	Debug(context.Context, string, ...Field)
	// Info 记录正常运行中的关键事实。
	Info(context.Context, string, ...Field)
	// Warn 记录可继续运行但需要关注的异常状态。
	Warn(context.Context, string, ...Field)
	// Error 记录由当前边界决定输出的失败。
	Error(context.Context, string, ...Field)
	// With 返回携带固定字段的派生 Logger。
	With(...Field) Logger
	// Named 返回附加逻辑命名空间的派生 Logger。
	Named(string) Logger
}
