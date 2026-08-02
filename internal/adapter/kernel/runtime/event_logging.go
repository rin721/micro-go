// 本文件集中发布 Runtime 事件，并隔离 Kernel Logger 与 Observer 的 panic 边界。
package runtime

import (
	"context"
	"errors"

	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
	"github.com/rin721/micro-go/internal/kernel/diagnostic"
	capabilitylogging "github.com/rin721/micro-go/types/capability/logging"
)

// publish 先写入不可缺失的 Kernel Logger，再同步通知可选 Observer，并合并两侧失败。
func (r *Runtime) publish(observer kernelapp.Observer, event kernelapp.Event) error {
	// 日志调用拥有独立 panic 边界，任何实现错误都会转换为 Logging 阶段错误。
	loggerErr := invokeDiagnostic(diagnostic.Logging, func() {
		// 每条事件至少携带序号、种类和状态，其他字段按是否存在追加。
		fields := []capabilitylogging.Field{
			{Key: "sequence", Value: event.Sequence},
			capabilitylogging.String("kind", string(event.Kind)),
			capabilitylogging.String("state", event.State.String()),
		}
		if event.Phase != "" {
			fields = append(fields, capabilitylogging.String("phase", string(event.Phase)))
		}
		// Module 只在组件或模块相关事件中存在，避免输出无意义空字段。
		if event.Module != "" {
			fields = append(fields, capabilitylogging.String("module", event.Module))
		}
		// Component 记录具体类型字符串，与 Module 共同定位生命周期对象。
		if event.Component != "" {
			fields = append(fields, capabilitylogging.String("component", event.Component))
		}
		// 错误只以脱敏文本写日志，原始错误仍保留在 Event 值和返回错误链中。
		if event.Err != nil {
			fields = append(fields, capabilitylogging.String("error", diagnostic.Redact(event.Err.Error())))
		}
		// Manager 在 publish 期间动态选择基线或增强 Logger。
		logger := r.dependencies.Logger
		// 严重失败、重启要求、关键里程碑和内部细节分别映射到稳定日志级别。
		switch {
		case event.Kind == kernelapp.ConfigurationFail || event.Kind == kernelapp.RunnerFailed || event.State == kernelapp.Failed:
			logger.Error(context.Background(), "runtime event", fields...)
		case event.State == kernelapp.RestartRequired:
			logger.Warn(context.Background(), "runtime event", fields...)
		case event.Kind == kernelapp.ConfigurationLoad || event.State == kernelapp.Running || event.State == kernelapp.Closed:
			logger.Info(context.Background(), "runtime event", fields...)
		default:
			logger.Debug(context.Background(), "runtime event", fields...)
		}
	})

	// 未配置 Observer 时不创建额外工作；日志错误仍会在最终 Join 中返回。
	var observerErr error
	if observer != nil {
		// Observer 与 Logger 使用独立 panic 边界，两个错误都不会互相覆盖。
		observerErr = invokeDiagnostic(diagnostic.Observe, func() { observer.Observe(event) })
	}
	// errors.Join 会忽略 nil，任一诊断边界失败都会由调用阶段完整感知。
	return errors.Join(loggerErr, observerErr)
}

// invokeDiagnostic 把诊断回调 panic 转换为带阶段的 ComponentError。
func invokeDiagnostic(phase diagnostic.Phase, call func()) (err error) {
	// defer 必须位于 call 同一 goroutine，才能 recover 并捕获准确堆栈。
	defer func() {
		if value := recover(); value != nil {
			err = &diagnostic.ComponentError{Phase: phase, Cause: diagnostic.NewPanicError(value)}
		}
	}()
	// 正常返回不产生错误；回调本身没有 error 返回值，panic 是唯一失败通道。
	call()
	return nil
}
