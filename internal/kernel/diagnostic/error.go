// Package diagnostic 定义 Kernel 各阶段共享的稳定错误模型。
package diagnostic

import (
	"fmt"
	"runtime/debug"
)

// Phase 标识错误或观察事件发生的 Kernel 阶段。它是项目自有类型，调用方不需要识别
// Dig、Koanf 或其他第三方库的错误分类。
type Phase string

const (
	// ModuleRegister 表示模块声明收集阶段。
	ModuleRegister Phase = "ModuleRegister"
	// ConfigLoad 表示读取并合并配置源阶段。
	ConfigLoad Phase = "ConfigLoad"
	// ConfigDecode 表示强类型配置解码阶段。
	ConfigDecode Phase = "ConfigDecode"
	// ConfigValidate 表示配置解码后的约束校验阶段。
	ConfigValidate Phase = "ConfigValidate"
	// Reload 表示已验证候选交给运行中组件应用的阶段。
	Reload Phase = "Reload"
	// GraphCompile 表示依赖图静态编译阶段。
	GraphCompile Phase = "GraphCompile"
	// Construct 表示组件实例构造阶段。
	Construct Phase = "Construct"
	// Prepare 表示生命周期准备阶段。
	Prepare Phase = "Prepare"
	// Start 表示生命周期启动阶段。
	Start Phase = "Start"
	// Run 表示 Runner 并发运行阶段。
	Run Phase = "Run"
	// Stop 表示已启动组件的停止阶段。
	Stop Phase = "Stop"
	// Close 表示全部已构造组件的资源释放阶段。
	Close Phase = "Close"
	// Logging 表示 Kernel 必有日志回调阶段。
	Logging Phase = "Logging"
	// Observe 表示 Observer 回调阶段。
	Observe Phase = "Observe"
)

// ComponentError 给业务错误补充模块、组件、Provider 和阶段上下文。
// Cause 保留原始业务错误，调用方仍可使用 errors.Is/As 判断；第三方引擎错误则在
// 适配层先归一化，避免实现细节泄漏。
type ComponentError struct {
	// Module 是组件所属模块；全局阶段失败时可以为空。
	Module string
	// Component 是组件具体类型字符串。
	Component string
	// Provider 是构造函数名称。
	Provider string
	// Phase 是错误发生的 Kernel 阶段。
	Phase Phase
	// Cause 是可供 errors.Is/As 检查的底层原因。
	Cause error
}

// Error 返回适合日志和诊断的稳定错误摘要。
func (e *ComponentError) Error() string {
	// 即使部分上下文字段为空也保留固定键，便于日志和测试稳定解析。
	return fmt.Sprintf("phase=%s module=%s component=%s provider=%s: %v", e.Phase, e.Module, e.Component, e.Provider, e.Cause)
}

// Unwrap 暴露 Cause，以保留标准库错误链语义。
func (e *ComponentError) Unwrap() error { return e.Cause }

// PanicError 把 panic 值与捕获时的堆栈转换成普通错误，使 Runtime 可以进入统一的
// 回滚和关闭流程，而不是让进程在边界处直接崩溃。
type PanicError struct {
	// Value 是 recover 捕获的原始 panic 值。
	Value any
	// Stack 是捕获边界处的 goroutine 堆栈。
	Stack []byte
}

// NewPanicError 在当前 goroutine 捕获堆栈并创建 PanicError。
func NewPanicError(value any) *PanicError {
	// 必须在 recover 所在 goroutine 立即取栈，否则后续阶段无法还原 panic 现场。
	return &PanicError{Value: value, Stack: debug.Stack()}
}

// Error 返回 panic 值的简要描述；完整定位信息保留在 Stack 中。
func (e *PanicError) Error() string { return fmt.Sprintf("panic: %v", e.Value) }
