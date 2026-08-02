// Package runtime 的重载测试直接验证候选 Snapshot 的提交边界。
package runtime

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	koanfadapter "github.com/rin721/micro-go/internal/adapter/kernel/config/koanf"
	configsource "github.com/rin721/micro-go/internal/adapter/kernel/config/source"
	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiled"
	kernelslog "github.com/rin721/micro-go/internal/adapter/kernel/logging/slog"
	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/reload"
	"github.com/rin721/micro-go/internal/kernel/testkit"
)

// reloadConfig 是主要候选配置类型。
type reloadConfig struct {
	// Value 是 Reloader 应用的必填值。
	Value string `yaml:"value" validate:"required"`
}

// reloadApplication 构造只包含 reload 所需字段的 Application 测试夹具。
func reloadApplication(configs []compiled.Config, instances []compiled.Instance, values map[string]any) *Application {
	// Logger 丢弃诊断，真实 Koanf Loader 保证候选行为与生产一致。
	return &Application{
		plan:      &compiled.Plan{Configs: configs},
		instances: instances,
		snapshot:  config.NewSnapshot(1, time.Now(), nil),
		options:   options{sources: []config.Source{configsource.FromValues(values)}, reloadTimeout: time.Second},
		runtime:   &Runtime{dependencies: Dependencies{Logger: kernelslog.New(io.Discard), Loader: koanfadapter.New()}},
	}
}

// reloadable 原地保存候选值并实现 Reloader。
type reloadable struct {
	// value 是当前已应用配置。
	value string
}

// Reload 读取强类型配置，成功时更新当前值。
func (r *reloadable) Reload(_ context.Context, snapshot config.Snapshot) (reload.Result, error) {
	candidate, err := config.Value[reloadConfig](snapshot)
	if err == nil {
		r.value = candidate.Value
	}
	return reload.Applied, err
}

// TestReloadRejectsInvalidCandidateWithoutPromotingSnapshot 验证普通校验失败只拒绝候选并发布事件。
func TestReloadRejectsInvalidCandidateWithoutPromotingSnapshot(t *testing.T) {
	// 空 reload 子树违反 Value required，当前版本初始为 1。
	typeOf := reflect.TypeOf(reloadConfig{})
	observer := &testkit.RecorderObserver{}
	a := reloadApplication([]compiled.Config{{Module: "test", Path: "reload", Type: typeOf}}, nil, map[string]any{"reload": map[string]any{}})
	a.options.observer = observer
	if err := a.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a.snapshot.Version != 1 {
		t.Fatalf("snapshot version = %d, want 1", a.snapshot.Version)
	}
	// 候选错误被观察为单个 ConfigurationFail，而 reload 本身返回 nil 继续运行。
	events := observer.Events()
	if len(events) != 1 || events[0].Kind != kernelapp.ConfigurationFail {
		t.Fatalf("events = %+v", events)
	}
}

// noReloadComponent 直接依赖配置但故意不实现 Reloader。
type noReloadComponent struct{}

// TestReloadRequestsRestartWhenAffectedComponentCannotReload 验证不能原地更新时保守请求重启。
func TestReloadRequestsRestartWhenAffectedComponentCannotReload(t *testing.T) {
	// Provider 依赖元数据明确标记组件受 reloadConfig 变化影响。
	typeOf := reflect.TypeOf(reloadConfig{})
	component := &noReloadComponent{}
	provider := compiled.Provider{Type: reflect.TypeOf(component), Dependencies: []compiled.Dependency{{Requested: typeOf, Resolved: typeOf, Config: true}}}
	a := reloadApplication(
		[]compiled.Config{{Module: "test", Path: "reload", Type: typeOf}},
		[]compiled.Instance{{Provider: provider, Value: component}},
		map[string]any{"reload": map[string]any{"value": "new"}},
	)
	if err := a.reload(context.Background()); !errors.Is(err, kernelapp.ErrRestartRequired) {
		t.Fatalf("reload() error = %v", err)
	}
	if a.snapshot.Version != 1 {
		t.Fatalf("snapshot version = %d, want 1", a.snapshot.Version)
	}
}

// secondReloadConfig 为第二个独立组件提供配置，制造部分应用失败。
type secondReloadConfig struct {
	// Value 是第二组件的必填候选值。
	Value string `yaml:"value" validate:"required"`
}

// failingReloader 固定返回注入错误。
type failingReloader struct {
	// cause 是测试需要在最终错误链中识别的哨兵。
	cause error
}

// Reload 拒绝候选并返回哨兵错误。
func (r *failingReloader) Reload(context.Context, config.Snapshot) (reload.Result, error) {
	return reload.Ignored, r.cause
}

// TestReloadPartialApplicationFailureKeepsOldSnapshotAndReturnsError 记录当前非回滚式组件应用边界。
func TestReloadPartialApplicationFailureKeepsOldSnapshotAndReturnsError(t *testing.T) {
	// first 会先应用 new，second 随后失败；Application 快照仍必须保持版本 1。
	firstType := reflect.TypeOf(reloadConfig{})
	secondType := reflect.TypeOf(secondReloadConfig{})
	first := &reloadable{value: "old"}
	sentinel := errors.New("second reload failed")
	second := &failingReloader{cause: sentinel}
	a := reloadApplication(
		[]compiled.Config{{Module: "first", Path: "first", Type: firstType}, {Module: "second", Path: "second", Type: secondType}},
		[]compiled.Instance{
			{Provider: compiled.Provider{Module: "first", Type: reflect.TypeOf(first), Dependencies: []compiled.Dependency{{Requested: firstType, Resolved: firstType, Config: true}}}, Value: first},
			{Provider: compiled.Provider{Module: "second", Type: reflect.TypeOf(second), Dependencies: []compiled.Dependency{{Requested: secondType, Resolved: secondType, Config: true}}}, Value: second},
		},
		map[string]any{"first": map[string]any{"value": "new"}, "second": map[string]any{"value": "new"}},
	)
	err := a.reload(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("reload() error = %v", err)
	}
	if first.value != "new" || a.snapshot.Version != 1 {
		// 断言明确区分组件已部分更新与 Runtime 未提升候选快照。
		t.Fatalf("partial result value=%q version=%d", first.value, a.snapshot.Version)
	}
}

// deadlineReloader 协作等待 Reload Context 超时并返回其原因。
type deadlineReloader struct{}

// Reload 阻塞到 Context 完成。
func (*deadlineReloader) Reload(ctx context.Context, _ config.Snapshot) (reload.Result, error) {
	<-ctx.Done()
	return reload.Ignored, ctx.Err()
}

// deadlineLoader 等待候选预算耗尽后故意返回 nil error。
type deadlineLoader struct{}

// Load 模拟忽略超时结果但最终响应 Context 的 Loader。
func (deadlineLoader) Load(ctx context.Context, _ uint64, _ []config.Source, _ []compiled.Config) (config.Loaded, error) {
	<-ctx.Done()
	return config.Loaded{}, nil
}

// TestReloadDetectsCandidateLoaderExceedingBudget 防止超时后返回 nil 的 Loader 被当成成功。
func TestReloadDetectsCandidateLoaderExceedingBudget(t *testing.T) {
	a := reloadApplication(nil, nil, nil)
	a.options.reloadTimeout = 20 * time.Millisecond
	a.runtime.dependencies.Loader = deadlineLoader{}
	err := a.reload(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) || a.snapshot.Version != 1 {
		t.Fatalf("reload() error=%v version=%d", err, a.snapshot.Version)
	}
}

// TestReloadUsesCooperativeTimeout 验证 Reloader 共享总预算并传播 DeadlineExceeded。
func TestReloadUsesCooperativeTimeout(t *testing.T) {
	typeOf := reflect.TypeOf(reloadConfig{})
	component := &deadlineReloader{}
	provider := compiled.Provider{Module: "test", Type: reflect.TypeOf(component), Dependencies: []compiled.Dependency{{Requested: typeOf, Resolved: typeOf, Config: true}}}
	a := reloadApplication(
		[]compiled.Config{{Module: "test", Path: "reload", Type: typeOf}},
		[]compiled.Instance{{Provider: provider, Value: component}},
		map[string]any{"reload": map[string]any{"value": "new"}},
	)
	a.options.reloadTimeout = 20 * time.Millisecond
	err := a.reload(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("reload() error = %v", err)
	}
}

// TestReloadPromotesValidatedSnapshot 确认组件成功应用候选后 Application 才提升版本，
// 从而避免组件看到新配置而 Runtime 仍报告旧快照。
func TestReloadPromotesValidatedSnapshot(t *testing.T) {
	// 单个受影响组件成功 Applied 后，value 和 Snapshot 版本应一起前进。
	typeOf := reflect.TypeOf(reloadConfig{})
	component := &reloadable{value: "old"}
	provider := compiled.Provider{Type: reflect.TypeOf(component), Dependencies: []compiled.Dependency{{Requested: typeOf, Resolved: typeOf, Config: true}}}
	a := &Application{
		plan:      &compiled.Plan{Configs: []compiled.Config{{Module: "test", Path: "reload", Type: typeOf}}},
		instances: []compiled.Instance{{Provider: provider, Value: component}},
		snapshot:  config.NewSnapshot(1, time.Now(), nil),
		options:   options{sources: []config.Source{configsource.FromValues(map[string]any{"reload": map[string]any{"value": "new"}})}, reloadTimeout: time.Second},
		runtime:   &Runtime{dependencies: Dependencies{Logger: kernelslog.New(io.Discard), Loader: koanfadapter.New()}},
	}
	if err := a.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if component.value != "new" || a.snapshot.Version != 2 {
		t.Fatalf("reload result value=%q version=%d", component.value, a.snapshot.Version)
	}
}
