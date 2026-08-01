// Package runtime 的重载测试直接验证候选 Snapshot 的提交边界。
package runtime

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	koanfadapter "github.com/rin721/micro-go/internal/adapter/kernel/config/koanf"
	configsource "github.com/rin721/micro-go/internal/adapter/kernel/config/source"
	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiled"
	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/reload"
	"github.com/rin721/micro-go/internal/kernel/testkit"
)

type reloadConfig struct {
	Value string `yaml:"value" validate:"required"`
}

func reloadApplication(configs []compiled.Config, instances []compiled.Instance, values map[string]any) *Application {
	return &Application{
		plan:      &compiled.Plan{Configs: configs},
		instances: instances,
		snapshot:  config.NewSnapshot(1, time.Now(), nil),
		options:   options{sources: []config.Source{configsource.FromValues(values)}, reloadTimeout: time.Second},
		runtime:   &Runtime{dependencies: Dependencies{Loader: koanfadapter.New()}},
	}
}

type reloadable struct{ value string }

func (r *reloadable) Reload(_ context.Context, snapshot config.Snapshot) (reload.Result, error) {
	candidate, err := config.Value[reloadConfig](snapshot)
	if err == nil {
		r.value = candidate.Value
	}
	return reload.Applied, err
}

func TestReloadRejectsInvalidCandidateWithoutPromotingSnapshot(t *testing.T) {
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
	events := observer.Events()
	if len(events) != 1 || events[0].Kind != kernelapp.ConfigurationFail {
		t.Fatalf("events = %+v", events)
	}
}

type noReloadComponent struct{}

func TestReloadRequestsRestartWhenAffectedComponentCannotReload(t *testing.T) {
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

type secondReloadConfig struct {
	Value string `yaml:"value" validate:"required"`
}

type failingReloader struct{ cause error }

func (r *failingReloader) Reload(context.Context, config.Snapshot) (reload.Result, error) {
	return reload.Ignored, r.cause
}

func TestReloadPartialApplicationFailureKeepsOldSnapshotAndReturnsError(t *testing.T) {
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
		t.Fatalf("partial result value=%q version=%d", first.value, a.snapshot.Version)
	}
}

type deadlineReloader struct{}

func (*deadlineReloader) Reload(ctx context.Context, _ config.Snapshot) (reload.Result, error) {
	<-ctx.Done()
	return reload.Ignored, ctx.Err()
}

type deadlineLoader struct{}

func (deadlineLoader) Load(ctx context.Context, _ uint64, _ []config.Source, _ []compiled.Config) (config.Loaded, error) {
	<-ctx.Done()
	return config.Loaded{}, nil
}

func TestReloadDetectsCandidateLoaderExceedingBudget(t *testing.T) {
	a := reloadApplication(nil, nil, nil)
	a.options.reloadTimeout = 20 * time.Millisecond
	a.runtime.dependencies.Loader = deadlineLoader{}
	err := a.reload(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) || a.snapshot.Version != 1 {
		t.Fatalf("reload() error=%v version=%d", err, a.snapshot.Version)
	}
}

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
	typeOf := reflect.TypeOf(reloadConfig{})
	component := &reloadable{value: "old"}
	provider := compiled.Provider{Type: reflect.TypeOf(component), Dependencies: []compiled.Dependency{{Requested: typeOf, Resolved: typeOf, Config: true}}}
	a := &Application{
		plan:      &compiled.Plan{Configs: []compiled.Config{{Module: "test", Path: "reload", Type: typeOf}}},
		instances: []compiled.Instance{{Provider: provider, Value: component}},
		snapshot:  config.NewSnapshot(1, time.Now(), nil),
		options:   options{sources: []config.Source{configsource.FromValues(map[string]any{"reload": map[string]any{"value": "new"}})}, reloadTimeout: time.Second},
		runtime:   &Runtime{dependencies: Dependencies{Loader: koanfadapter.New()}},
	}
	if err := a.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if component.value != "new" || a.snapshot.Version != 2 {
		t.Fatalf("reload result value=%q version=%d", component.value, a.snapshot.Version)
	}
}
