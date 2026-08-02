// 本文件验证 Kernel Logger 的必填装配、早期诊断、panic 隔离、显式替换和关停恢复。
package runtime_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	configsource "github.com/rin721/micro-go/internal/adapter/kernel/config/source"
	kernelslog "github.com/rin721/micro-go/internal/adapter/kernel/logging/slog"
	app "github.com/rin721/micro-go/internal/adapter/kernel/runtime"
	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
	"github.com/rin721/micro-go/internal/kernel/module"
	zapadapter "github.com/rin721/micro-go/pkg/adapter/logging/zap"
	capabilitylogging "github.com/rin721/micro-go/types/capability/logging"
)

// TestNewRequiresKernelLogger 同时拒绝 nil 接口和包裹 nil 指针的 Manager。
func TestNewRequiresKernelLogger(t *testing.T) {
	// 完全空 Dependencies 首先报告 Logger 缺失。
	if _, err := app.New(app.Dependencies{}); err == nil || !strings.Contains(err.Error(), "logger is nil") {
		t.Fatalf("New() error = %v, want logger error", err)
	}
	// typed nil 装入接口后接口本身非 nil，Runtime 必须用反射继续识别。
	var typedNil *kernelslog.Logger
	if _, err := app.New(app.Dependencies{Logger: typedNil}); err == nil || !strings.Contains(err.Error(), "logger is nil") {
		t.Fatalf("New() typed nil error = %v, want logger error", err)
	}
}

// TestKernelLoggerRecordsEarlyFailureWithoutBusinessLogger 验证注册失败由基线记录且敏感值脱敏。
func TestKernelLoggerRecordsEarlyFailureWithoutBusinessLogger(t *testing.T) {
	// Compile 尚未构造任何业务 Logger，输出只能来自 Kernel 基线。
	var output bytes.Buffer
	manager := kernelslog.New(&output)
	_, err := newRuntimeWithLogger(t, manager).Compile(app.WithModules(failingRegistrationModule{}))
	if err == nil {
		t.Fatal("Compile() succeeded")
	}
	logged := output.String()
	// 原始 token 和 password 值都不得进入诊断。
	for _, forbidden := range []string{"token=visible", "also-visible"} {
		if strings.Contains(logged, forbidden) {
			t.Fatalf("kernel logger leaked %q in %s", forbidden, logged)
		}
	}
	// 失败状态、脱敏占位和非敏感原因必须同时可见。
	for _, required := range []string{`"state":"Failed"`, "[REDACTED]", "registration failed"} {
		if !strings.Contains(logged, required) {
			t.Fatalf("kernel log %s does not contain %q", logged, required)
		}
	}
}

// TestKernelLoggerPanicIsIsolatedAndObserverStillRuns 验证 Logger panic 不阻止独立 Observer 回调。
func TestKernelLoggerPanicIsIsolatedAndObserverStillRuns(t *testing.T) {
	// countingObserver 记录同一事件仍被第二诊断边界调用。
	observer := &countingObserver{}
	_, err := newRuntimeWithLogger(t, panicManager{}).Compile(app.WithObserver(observer))
	if err == nil || !strings.Contains(err.Error(), "phase=Logging") || !strings.Contains(err.Error(), "panic: logger failed") {
		t.Fatalf("Compile() error = %v, want isolated logging panic", err)
	}
	if observer.count != 1 {
		t.Fatalf("Observer count = %d, want 1", observer.count)
	}
}

// failingRegistrationModule 返回包含敏感赋值的注册错误。
type failingRegistrationModule struct{}

// Name 返回失败模块名。
func (failingRegistrationModule) Name() string { return "failing-registration" }

// Register 固定返回需要脱敏的测试错误。
func (failingRegistrationModule) Register(module.Registry) error {
	return errors.New("registration failed token=visible password: also-visible")
}

// panicManager 的所有写日志方法都会 panic，用于检查 Logging 边界。
type panicManager struct{}

// Debug 注入日志 panic。
func (panicManager) Debug(context.Context, string, ...capabilitylogging.Field) {
	panic("logger failed")
}

// Info 注入日志 panic。
func (panicManager) Info(context.Context, string, ...capabilitylogging.Field) { panic("logger failed") }

// Warn 注入日志 panic。
func (panicManager) Warn(context.Context, string, ...capabilitylogging.Field) { panic("logger failed") }

// Error 注入日志 panic。
func (panicManager) Error(context.Context, string, ...capabilitylogging.Field) {
	panic("logger failed")
}

// With 保持同一 panic Manager。
func (p panicManager) With(...capabilitylogging.Field) capabilitylogging.Logger { return p }

// Named 保持同一 panic Manager。
func (p panicManager) Named(string) capabilitylogging.Logger { return p }

// Replace 在该场景中不改变行为。
func (panicManager) Replace(capabilitylogging.Logger) error { return nil }

// Restore 在该场景中无状态可恢复。
func (panicManager) Restore() {}

// countingObserver 统计 Observe 被调用的次数。
type countingObserver struct {
	// count 是同步调用计数。
	count int
}

// Observe 为每个事件增加计数。
func (o *countingObserver) Observe(kernelapp.Event) { o.count++ }

// TestKernelLoggerReplacementRequiresMatchingBinding 覆盖替换类型的四种静态无效声明。
func TestKernelLoggerReplacementRequiresMatchingBinding(t *testing.T) {
	// 接口类型不是具体 Provider 结果，Option 自身立即拒绝。
	_, err := newRuntime(t).Compile(app.WithKernelLoggerReplacement[capabilitylogging.Logger]())
	if err == nil || !strings.Contains(err.Error(), "must be a concrete type") {
		t.Fatalf("interface replacement error = %v", err)
	}

	// 具体类型没有 Provider 时由 Plan 校验拒绝。
	_, err = newRuntime(t).Compile(app.WithKernelLoggerReplacement[*replacementLogger]())
	if err == nil || !strings.Contains(err.Error(), "has no provider") {
		t.Fatalf("missing replacement error = %v", err)
	}

	// 有 Provider 但没有 logging.Logger Binding 仍不能成为委托目标。
	_, err = newRuntime(t).Compile(
		app.WithModules(unboundReplacementModule{}),
		app.WithKernelLoggerReplacement[*replacementLogger](),
	)
	if err == nil || !strings.Contains(err.Error(), "is not bound") {
		t.Fatalf("unbound replacement error = %v", err)
	}

	// 同一次 Build 重复选择替换类型不能静默覆盖前一个 Option。
	_, err = newRuntime(t).Compile(
		app.WithKernelLoggerReplacement[*replacementLogger](),
		app.WithKernelLoggerReplacement[*replacementLogger](),
	)
	if err == nil || !strings.Contains(err.Error(), "already set") {
		t.Fatalf("duplicate replacement error = %v", err)
	}
}

// TestKernelLoggerReplacementIsRestoredBeforeShutdown 验证增强 Logger 只接收 Built 到 Running 事件。
func TestKernelLoggerReplacementIsRestoredBeforeShutdown(t *testing.T) {
	// baseline 和 replacement 分别记录切换前后事件流。
	var baseline bytes.Buffer
	manager := kernelslog.New(&baseline)
	replacement := &replacementLogger{}
	runtime := newRuntimeWithLogger(t, manager)
	application, err := runtime.Build(context.Background(),
		app.WithModules(replacementModule{logger: replacement}),
		app.WithKernelLoggerReplacement[*replacementLogger](),
		app.WithStartupTimeout(time.Second),
		app.WithShutdownTimeout(time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(replacement.String(), "Built") {
		t.Fatalf("replacement did not receive Built event: %s", replacement.String())
	}

	// 启动应用并轮询原子 State，确认增强 Logger 处于活动期。
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	deadline := time.Now().Add(time.Second)
	for application.State() != kernelapp.Running {
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("application did not reach Running, state=%s", application.State())
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// replacement 作为组件实例由 Runtime Close，但恢复目标不等于关闭目标。
	if !replacement.closed {
		t.Fatal("Runtime did not close replacement instance")
	}
	// Stopping/Closed 必须由已恢复的基线记录，替换实例不再接收关停诊断。
	if strings.Contains(replacement.String(), "Stopping") || strings.Contains(replacement.String(), "Closed") {
		t.Fatalf("replacement received shutdown event after Restore: %s", replacement.String())
	}
	if !strings.Contains(baseline.String(), "Stopping") || !strings.Contains(baseline.String(), "Closed") {
		t.Fatalf("baseline did not receive shutdown events: %s", baseline.String())
	}
}

// TestConstructionFailureDoesNotSwitchKernelLogger 验证全部构造成功前不切换基线。
func TestConstructionFailureDoesNotSwitchKernelLogger(t *testing.T) {
	// replacement 先构造，后续依赖 Logger 的 Provider 再失败，触发构造回滚。
	var baseline bytes.Buffer
	manager := kernelslog.New(&baseline)
	replacement := &replacementLogger{}
	application, err := newRuntimeWithLogger(t, manager).Build(context.Background(),
		app.WithModules(replacementModule{logger: replacement}, failingAfterLoggingModule{}),
		app.WithKernelLoggerReplacement[*replacementLogger](),
		app.WithShutdownTimeout(time.Second),
	)
	if application != nil || err == nil || !strings.Contains(err.Error(), "construction failed") {
		t.Fatalf("Build() = (%v, %v), want construction failure", application, err)
	}
	if !replacement.closed {
		t.Fatal("construction rollback did not close replacement instance")
	}
	// 未完成 Build 时 replacement 不应收到任何 Kernel 事件。
	if replacement.String() != "" {
		t.Fatalf("replacement received Kernel events before successful Build: %s", replacement.String())
	}
	// 构造失败详情仍由基线记录。
	if !strings.Contains(baseline.String(), "Failed") || !strings.Contains(baseline.String(), "construction failed") {
		t.Fatalf("baseline did not record construction failure: %s", baseline.String())
	}
}

// TestZapCanExplicitlyReplaceKernelLogger 通过真实文件证明任意合约实现可显式接管运行期事件。
func TestZapCanExplicitlyReplaceKernelLogger(t *testing.T) {
	// Zap 文件与 baseline Buffer 分开，便于识别切换时点。
	var baseline bytes.Buffer
	manager := kernelslog.New(&baseline)
	path := filepath.Join(t.TempDir(), "zap.log")
	application, err := newRuntimeWithLogger(t, manager).Build(context.Background(),
		app.WithModules(zapModule{}),
		app.WithConfigSources(configsource.FromValues(map[string]any{
			"logging": map[string]any{"level": "debug", "output": path, "development": false},
		})),
		app.WithKernelLoggerReplacement[*zapadapter.Logger](),
		app.WithStartupTimeout(time.Second),
		app.WithShutdownTimeout(time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	deadline := time.Now().Add(time.Second)
	for application.State() != kernelapp.Running {
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("application did not reach Running, state=%s", application.State())
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// 活动期 Built/Running 在 Zap 文件，关停 Stopping 回到 baseline。
	if !strings.Contains(string(output), "Built") || !strings.Contains(string(output), "Running") {
		t.Fatalf("Zap did not receive active Kernel events: %s", output)
	}
	if strings.Contains(string(output), "Stopping") || !strings.Contains(baseline.String(), "Stopping") {
		t.Fatalf("Kernel baseline was not restored before shutdown: zap=%s baseline=%s", output, baseline.String())
	}
}

// replacementLogger 是线程安全的可关闭测试 Logger，记录所有消息和字段。
type replacementLogger struct {
	// mu 保护 values 和 closed。
	mu sync.Mutex
	// values 按写入顺序保存文本片段。
	values []string
	// closed 标记生命周期 Close 已执行。
	closed bool
}

// Debug 记录 Debug 调用。
func (l *replacementLogger) Debug(_ context.Context, message string, fields ...capabilitylogging.Field) {
	l.add(message, fields)
}

// Info 记录 Info 调用。
func (l *replacementLogger) Info(_ context.Context, message string, fields ...capabilitylogging.Field) {
	l.add(message, fields)
}

// Warn 记录 Warn 调用。
func (l *replacementLogger) Warn(_ context.Context, message string, fields ...capabilitylogging.Field) {
	l.add(message, fields)
}

// Error 记录 Error 调用。
func (l *replacementLogger) Error(_ context.Context, message string, fields ...capabilitylogging.Field) {
	l.add(message, fields)
}

// With 返回同一实例，测试不关心派生字段持久化。
func (l *replacementLogger) With(...capabilitylogging.Field) capabilitylogging.Logger { return l }

// Named 返回同一实例，测试只检查事件内容。
func (l *replacementLogger) Named(string) capabilitylogging.Logger { return l }

// Close 在线程安全临界区标记资源释放。
func (l *replacementLogger) Close(context.Context) error {
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()
	return nil
}

// add 追加消息和转换后的结构化字段。
func (l *replacementLogger) add(message string, fields []capabilitylogging.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.values = append(l.values, message)
	// 字段保存为 key=value，方便测试按稳定片段断言。
	for _, field := range fields {
		l.values = append(l.values, field.Key+"="+fieldText(field.Value))
	}
}

// String 返回当前记录的空格连接快照。
func (l *replacementLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.values, " ")
}

// replacementModule 提供并导出预建 replacementLogger。
type replacementModule struct {
	// logger 是测试控制的唯一替换实例。
	logger *replacementLogger
}

// Name 返回替换日志模块名。
func (replacementModule) Name() string { return "replacement-logger" }

// Register 声明 Provider、Logger Binding 和 Export。
func (m replacementModule) Register(registry module.Registry) error {
	if err := module.Provide(registry, func() *replacementLogger { return m.logger }); err != nil {
		return err
	}
	if err := module.Bind[capabilitylogging.Logger, *replacementLogger](registry); err != nil {
		return err
	}
	return module.Export[capabilitylogging.Logger](registry)
}

// unboundReplacementModule 只提供 replacementLogger，不建立接口 Binding。
type unboundReplacementModule struct{}

// Name 返回未绑定替换模块名。
func (unboundReplacementModule) Name() string { return "unbound-replacement" }

// Register 只声明具体 Provider。
func (unboundReplacementModule) Register(registry module.Registry) error {
	return module.Provide(registry, func() *replacementLogger { return &replacementLogger{} })
}

// constructionFailure 是后续失败 Provider 的具体结果占位类型。
type constructionFailure struct{}

// failingAfterLoggingModule 依赖 Logger 后返回构造错误。
type failingAfterLoggingModule struct{}

// Name 返回构造失败模块名。
func (failingAfterLoggingModule) Name() string { return "failing-after-logging" }

// Register 声明固定返回 construction failed 的 Provider。
func (failingAfterLoggingModule) Register(registry module.Registry) error {
	return module.Provide(registry, func(capabilitylogging.Logger) (*constructionFailure, error) {
		return nil, errors.New("construction failed")
	})
}

// fieldText 为测试记录器提供稳定简化字段文本。
func fieldText(value any) string {
	// 字符串保留真实内容，其他值只需证明字段存在。
	if text, ok := value.(string); ok {
		return text
	}
	return "value"
}
