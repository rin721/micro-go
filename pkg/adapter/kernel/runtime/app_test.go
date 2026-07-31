// Package runtime_test 用小型组件图和故障注入验证构造事务、生命周期顺序、模块边界与监听重载。
package runtime_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/module"
	"github.com/rin721/micro-go/internal/kernel/reload"
	configsource "github.com/rin721/micro-go/pkg/adapter/kernel/config/source"
	app "github.com/rin721/micro-go/pkg/adapter/kernel/runtime"
	slogadapter "github.com/rin721/micro-go/pkg/adapter/logging/slog"
	zapadapter "github.com/rin721/micro-go/pkg/adapter/logging/zap"
	"github.com/rin721/micro-go/types/capability/logging"
)

type recorder struct {
	mu     sync.Mutex
	values []string
}
type eventSink interface{ add(string) }

func (r *recorder) add(value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values = append(r.values, value)
}
func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.values...)
}

type testConfig struct {
	Name string `yaml:"name" validate:"required"`
}
type serviceContract interface{ Name() string }
type service struct {
	cfg    testConfig
	events eventSink
}

func newService(cfg testConfig, events eventSink) *service {
	events.add("construct-service")
	return &service{cfg: cfg, events: events}
}
func (s *service) Name() string                  { return s.cfg.Name }
func (s *service) Prepare(context.Context) error { s.events.add("prepare-service"); return nil }
func (s *service) Start(context.Context) error   { s.events.add("start-service"); return nil }
func (s *service) Stop(context.Context) error    { s.events.add("stop-service"); return nil }
func (s *service) Close(context.Context) error   { s.events.add("close-service"); return nil }

type consumer struct {
	service serviceContract
	events  eventSink
}

func newConsumer(service serviceContract, events eventSink) *consumer {
	events.add("construct-consumer")
	return &consumer{service: service, events: events}
}
func (c *consumer) Prepare(context.Context) error { c.events.add("prepare-consumer"); return nil }
func (c *consumer) Start(context.Context) error   { c.events.add("start-consumer"); return nil }
func (c *consumer) Run(context.Context) error {
	c.events.add("run-consumer:" + c.service.Name())
	return nil
}
func (c *consumer) Stop(context.Context) error  { c.events.add("stop-consumer"); return nil }
func (c *consumer) Close(context.Context) error { c.events.add("close-consumer"); return nil }

type valuesModule struct{ events *recorder }

func (valuesModule) Name() string { return "values" }
func (m valuesModule) Register(reg module.Registry) error {
	if err := module.Provide(reg, func() *recorder { return m.events }); err != nil {
		return err
	}
	if err := module.Bind[eventSink, *recorder](reg); err != nil {
		return err
	}
	return module.Export[eventSink](reg)
}

type serviceModule struct{}

func (serviceModule) Name() string { return "service" }
func (serviceModule) Register(reg module.Registry) error {
	if err := module.Config[testConfig](reg, "service"); err != nil {
		return err
	}
	if err := module.Provide(reg, newService); err != nil {
		return err
	}
	if err := module.Bind[serviceContract, *service](reg); err != nil {
		return err
	}
	return module.Export[serviceContract](reg)
}

type consumerModule struct{}

func (consumerModule) Name() string                       { return "consumer" }
func (consumerModule) Register(reg module.Registry) error { return module.Provide(reg, newConsumer) }

// TestApplicationLifecycleAndGraph 同时验证稳定依赖图、正序启动和逆序关闭这一组核心不变量。
func TestApplicationLifecycleAndGraph(t *testing.T) {
	events := &recorder{}
	options := []app.Option{
		app.WithModules(valuesModule{events}, serviceModule{}, consumerModule{}),
		app.WithConfigSources(configsource.FromValues(map[string]any{"service": map[string]any{"name": "demo"}})),
		app.WithStartupTimeout(time.Second), app.WithShutdownTimeout(time.Second),
	}
	plan, err := newRuntime(t).Compile(options...)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	graph := plan.DependencyGraph()
	if len(graph.Nodes) != 4 || !strings.Contains(graph.DOT(), "serviceContract") {
		t.Fatalf("unexpected graph: %+v", graph)
	}
	application, err := newRuntime(t).Build(context.Background(), options...)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if err := application.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if application.State() != kernelapp.Closed {
		t.Fatalf("State() = %s, want Closed", application.State())
	}
	want := []string{"construct-service", "construct-consumer", "prepare-service", "prepare-consumer", "start-service", "start-consumer", "run-consumer:demo", "stop-consumer", "stop-service", "close-consumer", "close-service"}
	got := events.snapshot()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("events mismatch (-want +got):\n%s", diff)
	}
}

type closeable struct{ closed *bool }

func (c *closeable) Close(context.Context) error { *c.closed = true; return nil }

type rollbackModule struct{ closed *bool }

func (rollbackModule) Name() string { return "rollback" }
func (m rollbackModule) Register(reg module.Registry) error {
	if err := module.Provide(reg, func() *closeable { return &closeable{closed: m.closed} }); err != nil {
		return err
	}
	return module.Provide(reg, func(*closeable) (*consumer, error) { return nil, errors.New("boom") })
}

// TestBuildRollsBackConstructedComponents 注入后续 Provider 失败，确认此前构造的 Closer
// 被立即逆序释放且不会返回半构造 Application。
func TestBuildRollsBackConstructedComponents(t *testing.T) {
	closed := false
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(rollbackModule{&closed}))
	if err == nil || application != nil {
		t.Fatalf("Build() = (%v, %v), want nil,error", application, err)
	}
	if !closed {
		t.Fatal("constructed component was not closed")
	}
}

type concreteOwner struct{}

func (concreteOwner) Name() string { return "owner" }
func (concreteOwner) Register(reg module.Registry) error {
	return module.Provide(reg, func() *service { return &service{} })
}

type concreteConsumer struct{}

func (concreteConsumer) Name() string { return "concrete-consumer" }
func (concreteConsumer) Register(reg module.Registry) error {
	return module.Provide(reg, func(*service) *consumer { return &consumer{} })
}

// TestCompileRejectsCrossModuleConcreteDependency 保证模块之间只能通过显式导出的接口协作。
func TestCompileRejectsCrossModuleConcreteDependency(t *testing.T) {
	_, err := newRuntime(t).Compile(app.WithModules(concreteOwner{}, concreteConsumer{}))
	if err == nil || !strings.Contains(err.Error(), "private concrete type") {
		t.Fatalf("Compile() error = %v", err)
	}
}

type startA struct{ events *recorder }

func (a *startA) Start(context.Context) error { a.events.add("start-a"); return nil }
func (a *startA) Stop(context.Context) error  { a.events.add("stop-a"); return nil }
func (a *startA) Close(context.Context) error { a.events.add("close-a"); return nil }

type startB struct {
	a      *startA
	events *recorder
}

func (b *startB) Start(context.Context) error {
	b.events.add("start-b")
	return errors.New("start failed")
}
func (b *startB) Stop(context.Context) error  { b.events.add("stop-b"); return nil }
func (b *startB) Close(context.Context) error { b.events.add("close-b"); return nil }

type startFailureModule struct{ events *recorder }

func (startFailureModule) Name() string { return "start-failure" }
func (m startFailureModule) Register(reg module.Registry) error {
	if err := module.Provide(reg, func() *startA { return &startA{events: m.events} }); err != nil {
		return err
	}
	return module.Provide(reg, func(a *startA) *startB { return &startB{a: a, events: m.events} })
}

// TestStartFailureStopsOnlyStartedAndClosesAll 区分 Stop 与 Close 的所有权：只停止已启动组件，
// 但释放所有已构造组件。
func TestStartFailureStopsOnlyStartedAndClosesAll(t *testing.T) {
	events := &recorder{}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(startFailureModule{events}), app.WithShutdownTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	err = application.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "start failed") {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{"start-a", "start-b", "stop-a", "close-b", "close-a"}
	if diff := cmp.Diff(want, events.snapshot()); diff != "" {
		t.Fatalf("rollback mismatch (-want +got):\n%s", diff)
	}
}

type panicObserver struct{}

func (panicObserver) Observe(kernelapp.Event) { panic("observer failed") }

// TestObserverPanicIsConverted 验证诊断回调 panic 不会越过 Runtime 边界，而会转换为 PanicError。
func TestObserverPanicIsConverted(t *testing.T) {
	application, err := newRuntime(t).Build(context.Background(), app.WithObserver(panicObserver{}))
	if application != nil || err == nil || !strings.Contains(err.Error(), "panic: observer failed") {
		t.Fatalf("Build() = (%v,%v)", application, err)
	}
}

// TestCompileRejectsMultipleLoggingImplementations 确认 Runtime 不替组合根静默选择 Zap 或 Slog，
// 同一日志契约出现两个 Binding 时必须显式失败。
func TestCompileRejectsMultipleLoggingImplementations(t *testing.T) {
	_, err := newRuntime(t).Compile(app.WithModules(zapModule{}, slogModule{}))
	if err == nil || !strings.Contains(err.Error(), "bindings in both") {
		t.Fatalf("Compile() error = %v", err)
	}
}

type zapModule struct{}

func (zapModule) Name() string { return "logging-zap" }
func (zapModule) Register(registry module.Registry) error {
	if err := module.Config[zapadapter.Config](registry, "logging"); err != nil {
		return err
	}
	if err := module.Provide(registry, zapadapter.New); err != nil {
		return err
	}
	if err := module.Bind[logging.Logger, *zapadapter.Logger](registry); err != nil {
		return err
	}
	return module.Export[logging.Logger](registry)
}

type slogModule struct{}

func (slogModule) Name() string { return "logging-slog" }
func (slogModule) Register(registry module.Registry) error {
	if err := module.Config[slogadapter.Config](registry, "logging"); err != nil {
		return err
	}
	if err := module.Provide(registry, slogadapter.New); err != nil {
		return err
	}
	if err := module.Bind[logging.Logger, *slogadapter.Logger](registry); err != nil {
		return err
	}
	return module.Export[logging.Logger](registry)
}

type watchConfig struct {
	Value string `yaml:"value" validate:"required"`
}
type watchComponent struct {
	value   string
	started chan struct{}
	changed chan string
}

func (c *watchComponent) Run(ctx context.Context) error {
	close(c.started)
	<-ctx.Done()
	return ctx.Err()
}
func (c *watchComponent) Reload(_ context.Context, snapshot config.Snapshot) (reload.Result, error) {
	value, err := config.Value[watchConfig](snapshot)
	if err != nil {
		return reload.Ignored, err
	}
	c.value = value.Value
	c.changed <- value.Value
	return reload.Applied, nil
}

type watchModule struct{ component *watchComponent }

func (watchModule) Name() string { return "watch" }
func (m watchModule) Register(reg module.Registry) error {
	if err := module.Config[watchConfig](reg, "watch"); err != nil {
		return err
	}
	return module.Provide(reg, func(cfg watchConfig) *watchComponent { m.component.value = cfg.Value; return m.component })
}

// TestFileWatchReloadsRunningComponent 使用真实临时文件事件验证监听、去抖、候选重建和
// 组件 Reload 的完整运行链，并由 Context 驱动优雅退出。
func TestFileWatchReloadsRunningComponent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte("watch:\n  value: old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	component := &watchComponent{started: make(chan struct{}), changed: make(chan string, 1)}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(watchModule{component}), app.WithConfigSources(configsource.FromFile(path)), app.WithConfigWatch(), app.WithReloadDebounce(50*time.Millisecond), app.WithShutdownTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	select {
	case <-component.started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	if err := os.WriteFile(path, []byte("watch:\n  value: new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case value := <-component.changed:
		if value != "new" {
			t.Fatalf("reloaded value=%q", value)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("configuration was not reloaded")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("application did not stop")
	}
}
