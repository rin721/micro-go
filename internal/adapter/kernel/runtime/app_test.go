// Package runtime_test 用小型组件图和故障注入验证构造事务、生命周期顺序、模块边界与监听重载。
package runtime_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	configsource "github.com/rin721/micro-go/internal/adapter/kernel/config/source"
	app "github.com/rin721/micro-go/internal/adapter/kernel/runtime"
	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/diagnostic"
	"github.com/rin721/micro-go/internal/kernel/module"
	"github.com/rin721/micro-go/internal/kernel/reload"
	noopadapter "github.com/rin721/micro-go/pkg/adapter/logging/noop"
	zapadapter "github.com/rin721/micro-go/pkg/adapter/logging/zap"
	"github.com/rin721/micro-go/types/capability/logging"
)

// recorder 线程安全地保存生命周期标记，用于比较跨 goroutine 的完整顺序。
type recorder struct {
	// mu 保护 values 的追加和快照复制。
	mu sync.Mutex
	// values 按实际发生顺序保存事件文本。
	values []string
}

// eventSink 是测试组件依赖的最小事件记录契约。
type eventSink interface {
	// add 追加一条生命周期标记。
	add(string)
}

// add 在线程安全临界区追加事件。
func (r *recorder) add(value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values = append(r.values, value)
}

// snapshot 返回事件切片副本，断言不会修改记录器内部状态。
func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.values...)
}

// testConfig 是 serviceModule 独占的强类型测试配置。
type testConfig struct {
	// Name 会被 service 返回并由 consumer Runner 观察。
	Name string `yaml:"name" validate:"required"`
}

// serviceContract 是 serviceModule 向 consumerModule 导出的接口。
type serviceContract interface {
	// Name 返回配置注入的服务名称。
	Name() string
}

// service 实现配置、接口及 Prepare/Start/Stop/Close 生命周期。
type service struct {
	// cfg 保存构造期注入的强类型配置。
	cfg testConfig
	// events 记录每个生命周期调用。
	events eventSink
}

// newService 记录构造顺序并创建 service。
func newService(cfg testConfig, events eventSink) *service {
	events.add("construct-service")
	return &service{cfg: cfg, events: events}
}

// Name 实现导出的服务契约。
func (s *service) Name() string { return s.cfg.Name }

// Prepare 记录服务准备阶段。
func (s *service) Prepare(context.Context) error { s.events.add("prepare-service"); return nil }

// Start 记录服务启动阶段。
func (s *service) Start(context.Context) error { s.events.add("start-service"); return nil }

// Stop 记录服务停止阶段。
func (s *service) Stop(context.Context) error { s.events.add("stop-service"); return nil }

// Close 记录服务资源释放阶段。
func (s *service) Close(context.Context) error { s.events.add("close-service"); return nil }

// consumer 通过接口消费 service，并实现完整长期任务生命周期。
type consumer struct {
	// service 是跨模块导出的接口依赖。
	service serviceContract
	// events 与 service 共享同一个记录器实例。
	events eventSink
}

// newConsumer 记录构造顺序并保存接口依赖。
func newConsumer(service serviceContract, events eventSink) *consumer {
	events.add("construct-consumer")
	return &consumer{service: service, events: events}
}

// Prepare 记录消费者准备阶段。
func (c *consumer) Prepare(context.Context) error { c.events.add("prepare-consumer"); return nil }

// Start 记录消费者启动阶段。
func (c *consumer) Start(context.Context) error { c.events.add("start-consumer"); return nil }

// Run 记录配置名称后阻塞到 Context 取消。
func (c *consumer) Run(ctx context.Context) error {
	c.events.add("run-consumer:" + c.service.Name())
	<-ctx.Done()
	return ctx.Err()
}

// Stop 记录消费者停止阶段。
func (c *consumer) Stop(context.Context) error { c.events.add("stop-consumer"); return nil }

// Close 记录消费者资源释放阶段。
func (c *consumer) Close(context.Context) error { c.events.add("close-consumer"); return nil }

// valuesModule 提供并导出所有测试模块共享的 eventSink。
type valuesModule struct {
	// events 是测试预先创建的唯一记录器。
	events *recorder
}

// Name 返回记录模块的稳定名称。
func (valuesModule) Name() string { return "values" }

// Register 声明记录器 Provider、接口 Binding 和 Export。
func (m valuesModule) Register(reg module.Registry) error {
	if err := module.Provide(reg, func() *recorder { return m.events }); err != nil {
		return err
	}
	if err := module.Bind[eventSink, *recorder](reg); err != nil {
		return err
	}
	return module.Export[eventSink](reg)
}

// serviceModule 声明测试配置和 service 能力。
type serviceModule struct{}

// Name 返回服务模块的稳定名称。
func (serviceModule) Name() string { return "service" }

// Register 声明配置、Provider、Binding 和 Export。
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

// consumerModule 声明依赖公开 serviceContract 的最终 Runner。
type consumerModule struct{}

// Name 返回消费者模块的稳定名称。
func (consumerModule) Name() string { return "consumer" }

// Register 只登记 consumer Provider，不导出具体实现。
func (consumerModule) Register(reg module.Registry) error { return module.Provide(reg, newConsumer) }

// TestApplicationLifecycleAndGraph 同时验证稳定依赖图、正序启动和逆序关闭这一组核心不变量。
func TestApplicationLifecycleAndGraph(t *testing.T) {
	// 三个模块形成 recorder -> service -> consumer 的稳定依赖链。
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
	// 图包含一个配置节点和三个 Provider，DOT 应保留接口依赖标签。
	graph := plan.DependencyGraph()
	if len(graph.Nodes) != 4 || !strings.Contains(graph.DOT(), "serviceContract") {
		t.Fatalf("unexpected graph: %+v", graph)
	}
	application, err := newRuntime(t).Build(context.Background(), options...)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	// Run 独立执行，测试轮询记录器确认 Runner 真正进入长期循环。
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	deadline := time.Now().Add(time.Second)
	for {
		if slices.Contains(events.snapshot(), "run-consumer:demo") {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("consumer runner did not start")
		}
		time.Sleep(time.Millisecond)
	}
	// 取消后等待完整 Stop/Close，再检查最终状态和精确顺序。
	cancel()
	if err := <-done; err != nil {
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

// closeable 通过外部布尔指针暴露自己是否被回滚关闭。
type closeable struct {
	// closed 由 Close 设置，测试在 Build 返回后检查。
	closed *bool
}

// Close 标记资源已经释放。
func (c *closeable) Close(context.Context) error { *c.closed = true; return nil }

// rollbackModule 先构造 Closer，再让后续 Provider 返回错误。
type rollbackModule struct {
	// closed 由第一个实例共享给测试。
	closed *bool
}

// Name 返回构造回滚模块名。
func (rollbackModule) Name() string { return "rollback" }

// Register 声明一个成功 Provider 和一个必然失败的消费者 Provider。
func (m rollbackModule) Register(reg module.Registry) error {
	if err := module.Provide(reg, func() *closeable { return &closeable{closed: m.closed} }); err != nil {
		return err
	}
	return module.Provide(reg, func(*closeable) (*consumer, error) { return nil, errors.New("boom") })
}

// TestBuildRollsBackConstructedComponents 注入后续 Provider 失败，确认此前构造的 Closer
// 被立即逆序释放且不会返回半构造 Application。
func TestBuildRollsBackConstructedComponents(t *testing.T) {
	// closed 初值 false，只有 Runtime 回滚调用 Close 才会变为 true。
	closed := false
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(rollbackModule{&closed}))
	if err == nil || application != nil {
		t.Fatalf("Build() = (%v, %v), want nil,error", application, err)
	}
	// 构造失败不能返回 Application，并且已完成资源必须立即释放。
	if !closed {
		t.Fatal("constructed component was not closed")
	}
}

// errorCloseable 在 Close 时返回指定错误，用于验证多错误保留。
type errorCloseable struct {
	// cause 是清理阶段的哨兵错误。
	cause error
}

// Close 返回预置清理错误。
func (c *errorCloseable) Close(context.Context) error { return c.cause }

// cancellationAwareCloseable 只有在独立未取消 Context 下才完成关闭。
type cancellationAwareCloseable struct {
	// closed 标记实际执行了资源释放主体。
	closed bool
	// cause 是关闭后仍需返回的清理错误。
	cause error
}

// Close 先拒绝已取消 Context，再标记关闭并返回哨兵错误。
func (c *cancellationAwareCloseable) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.closed = true
	return c.cause
}

// cancellationRollbackModule 在首个构造函数中取消 Build Context。
type cancellationRollbackModule struct {
	// cancel 触发调用方构造 Context 取消。
	cancel context.CancelFunc
	// closeable 是已完成并需要 Runtime 回滚的实例。
	closeable *cancellationAwareCloseable
}

// Name 返回取消回滚模块名。
func (cancellationRollbackModule) Name() string { return "cancellation-rollback" }

// Register 声明取消构造器和其后不应完成的消费者。
func (m cancellationRollbackModule) Register(reg module.Registry) error {
	if err := module.Provide(reg, func() *cancellationAwareCloseable {
		m.cancel()
		return m.closeable
	}); err != nil {
		return err
	}
	return module.Provide(reg, func(*cancellationAwareCloseable) *consumer { return nil })
}

// TestBuildCancellationUsesIndependentRollbackContext 验证清理不复用已经取消的构造 Context。
func TestBuildCancellationUsesIndependentRollbackContext(t *testing.T) {
	// closeErr 与 context.Canceled 都必须保留在最终错误链中。
	closeErr := errors.New("rollback close failed")
	closeable := &cancellationAwareCloseable{cause: closeErr}
	ctx, cancel := context.WithCancel(context.Background())
	application, err := newRuntime(t).Build(ctx,
		app.WithModules(cancellationRollbackModule{cancel: cancel, closeable: closeable}),
		app.WithShutdownTimeout(time.Second),
	)
	if application != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, closeErr) {
		t.Fatalf("Build() = (%v, %v)", application, err)
	}
	// closed=true 证明 Runtime 创建了独立关停 Context 并实际进入 Close 主体。
	if !closeable.closed {
		t.Fatal("constructed resource was not closed with an independent context")
	}
}

// panicRollbackModule 先构造会关闭失败的资源，再触发 Provider panic。
type panicRollbackModule struct {
	// closeErr 是回滚阶段需要保留的哨兵错误。
	closeErr error
}

// Name 返回 panic 回滚模块名。
func (panicRollbackModule) Name() string { return "panic-rollback" }

// Register 声明成功资源 Provider 和 panic 消费者。
func (m panicRollbackModule) Register(reg module.Registry) error {
	if err := module.Provide(reg, func() *errorCloseable { return &errorCloseable{cause: m.closeErr} }); err != nil {
		return err
	}
	return module.Provide(reg, func(*errorCloseable) *consumer { panic("provider panic") })
}

// TestBuildPreservesProviderPanicAndRollbackError 验证 panic 与 Close 错误同时保留。
func TestBuildPreservesProviderPanicAndRollbackError(t *testing.T) {
	closeErr := errors.New("rollback close failed")
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(panicRollbackModule{closeErr: closeErr}))
	if application != nil || err == nil {
		t.Fatalf("Build() = (%v, %v)", application, err)
	}
	var panicErr *diagnostic.PanicError
	// errors.As 识别项目 PanicError，errors.Is 继续识别清理哨兵。
	if !errors.As(err, &panicErr) || !errors.Is(err, closeErr) {
		t.Fatalf("Build() error=%v", err)
	}
}

// concreteOwner 提供但不导出具体 service 类型。
type concreteOwner struct{}

// Name 返回具体类型所有者模块名。
func (concreteOwner) Name() string { return "owner" }

// Register 只登记私有具体 Provider。
func (concreteOwner) Register(reg module.Registry) error {
	return module.Provide(reg, func() *service { return &service{} })
}

// concreteConsumer 故意跨模块直接请求私有 service 类型。
type concreteConsumer struct{}

// Name 返回违规消费者模块名。
func (concreteConsumer) Name() string { return "concrete-consumer" }

// Register 声明跨模块具体依赖以触发 Compiler 边界错误。
func (concreteConsumer) Register(reg module.Registry) error {
	return module.Provide(reg, func(*service) *consumer { return &consumer{} })
}

// TestCompileRejectsCrossModuleConcreteDependency 保证模块之间只能通过显式导出的接口协作。
func TestCompileRejectsCrossModuleConcreteDependency(t *testing.T) {
	// 编译阶段应在构造前识别 private concrete type。
	_, err := newRuntime(t).Compile(app.WithModules(concreteOwner{}, concreteConsumer{}))
	if err == nil || !strings.Contains(err.Error(), "private concrete type") {
		t.Fatalf("Compile() error = %v", err)
	}
}

// startA 是成功启动并应在回滚中 Stop/Close 的依赖组件。
type startA struct {
	// events 记录回滚顺序。
	events *recorder
}

// Start 记录 A 成功启动。
func (a *startA) Start(context.Context) error { a.events.add("start-a"); return nil }

// Stop 记录 A 被停止。
func (a *startA) Stop(context.Context) error { a.events.add("stop-a"); return nil }

// Close 记录 A 被关闭。
func (a *startA) Close(context.Context) error { a.events.add("close-a"); return nil }

// startB 依赖 A，但自己的 Start 返回错误。
type startB struct {
	// a 建立 B 对 A 的构造依赖。
	a *startA
	// events 与 A 共享回滚记录器。
	events *recorder
}

// Start 记录 B 调用后返回预期失败。
func (b *startB) Start(context.Context) error {
	b.events.add("start-b")
	return errors.New("start failed")
}

// Stop 提供方法但不应被调用，因为 B 从未成功启动。
func (b *startB) Stop(context.Context) error { b.events.add("stop-b"); return nil }

// Close 必须被调用，因为 B 已经构造成功。
func (b *startB) Close(context.Context) error { b.events.add("close-b"); return nil }

// startFailureModule 构造 startA 和依赖它的 startB。
type startFailureModule struct {
	// events 保存共享顺序记录器。
	events *recorder
}

// Name 返回启动失败模块名。
func (startFailureModule) Name() string { return "start-failure" }

// Register 按依赖顺序声明两个组件。
func (m startFailureModule) Register(reg module.Registry) error {
	if err := module.Provide(reg, func() *startA { return &startA{events: m.events} }); err != nil {
		return err
	}
	return module.Provide(reg, func(a *startA) *startB { return &startB{a: a, events: m.events} })
}

// TestStartFailureStopsOnlyStartedAndClosesAll 区分 Stop 与 Close 的所有权：只停止已启动组件，
// 但释放所有已构造组件。
func TestStartFailureStopsOnlyStartedAndClosesAll(t *testing.T) {
	// Build 成功后 Run 会在 startB.Start 注入失败。
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
	// 期望没有 stop-b，但 Close 仍覆盖 B 和 A，并保持逆序。
	if diff := cmp.Diff(want, events.snapshot()); diff != "" {
		t.Fatalf("rollback mismatch (-want +got):\n%s", diff)
	}
}

// panicObserver 在任意事件上 panic，用于验证诊断边界。
type panicObserver struct{}

// Observe 固定触发测试 panic。
func (panicObserver) Observe(kernelapp.Event) { panic("observer failed") }

// TestObserverPanicIsConverted 验证诊断回调 panic 不会越过 Runtime 边界，而会转换为 PanicError。
func TestObserverPanicIsConverted(t *testing.T) {
	// Build 的首个 Registering 事件即触发 panic，因此不能返回 Application。
	application, err := newRuntime(t).Build(context.Background(), app.WithObserver(panicObserver{}))
	if application != nil || err == nil || !strings.Contains(err.Error(), "panic: observer failed") {
		t.Fatalf("Build() = (%v,%v)", application, err)
	}
}

// TestCompileRejectsMultipleLoggingImplementations 确认 Runtime 不替组合根静默选择 Zap 或 Noop，
// 同一日志契约出现两个 Binding 时必须显式失败。
func TestCompileRejectsMultipleLoggingImplementations(t *testing.T) {
	// 两个模块都绑定并导出同一 Logger 接口，Compiler 必须要求组合根唯一选择。
	_, err := newRuntime(t).Compile(app.WithModules(zapModule{}, noopModule{}))
	if err == nil || !strings.Contains(err.Error(), "bindings in both") {
		t.Fatalf("Compile() error = %v", err)
	}
}

// zapModule 是测试用 Zap 配置、Provider、Binding 和 Export 声明。
type zapModule struct{}

// Name 返回 Zap 模块名。
func (zapModule) Name() string { return "logging-zap" }

// Register 声明 Zap 日志完整能力。
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

// noopModule 是测试用 Noop Provider、Binding 和 Export 声明。
type noopModule struct{}

// Name 返回 Noop 模块名。
func (noopModule) Name() string { return "logging-noop" }

// Register 声明 Noop 日志能力。
func (noopModule) Register(registry module.Registry) error {
	if err := module.Provide(registry, noopadapter.New); err != nil {
		return err
	}
	if err := module.Bind[logging.Logger, *noopadapter.Logger](registry); err != nil {
		return err
	}
	return module.Export[logging.Logger](registry)
}

// watchConfig 是文件监听测试唯一配置值。
type watchConfig struct {
	// Value 用于判断 Runner 和 Reload 看到了哪个版本。
	Value string `yaml:"value" validate:"required"`
}

// watchComponent 记录当前配置，并通过通道同步 Runner 与测试 goroutine。
type watchComponent struct {
	// value 是构造或 Reload 设置的当前值。
	value string
	// started 在 Runner 进入后关闭。
	started chan struct{}
	// changed 接收 Reload 应用的新值。
	changed chan string
	// runValue 可选记录 Runner 启动瞬间观察到的值。
	runValue chan string
}

// Run 发布启动时配置、关闭 started 并等待取消。
func (c *watchComponent) Run(ctx context.Context) error {
	if c.runValue != nil {
		c.runValue <- c.value
	}
	close(c.started)
	<-ctx.Done()
	return ctx.Err()
}

// TestRunReconcilesConfigChangedBetweenBuildAndWatch 验证启动补偿 Reload 封闭监听竞态窗口。
func TestRunReconcilesConfigChangedBetweenBuildAndWatch(t *testing.T) {
	// Build 先读取 old，随后在 Run 建立 Watcher 前把文件改为 new。
	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte("watch:\n  value: old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileSource, err := configsource.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	component := &watchComponent{started: make(chan struct{}), changed: make(chan string, 1), runValue: make(chan string, 1)}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(watchModule{component}), app.WithConfigSources(fileSource), app.WithConfigWatch(), app.WithReloadDebounce(time.Millisecond), app.WithShutdownTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("watch:\n  value: new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Runner 启动时应直接观察补偿 Reload 后的 new，而不是 Build 快照 old。
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	select {
	case value := <-component.runValue:
		if value != "new" {
			t.Fatalf("runner observed %q, want reconciled value", value)
		}
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// Reload 从候选快照读取配置、更新当前值并通知测试。
func (c *watchComponent) Reload(_ context.Context, snapshot config.Snapshot) (reload.Result, error) {
	value, err := config.Value[watchConfig](snapshot)
	if err != nil {
		return reload.Ignored, err
	}
	c.value = value.Value
	// changed 带缓冲，Reload 不依赖测试 goroutine 恰好已开始接收。
	c.changed <- value.Value
	return reload.Applied, nil
}

// watchModule 把预建组件与 watchConfig 声明装入依赖图。
type watchModule struct {
	// component 是测试观察的唯一实例。
	component *watchComponent
}

// Name 返回监听测试模块名。
func (watchModule) Name() string { return "watch" }

// Register 声明配置，并用闭包把初始值写入预建组件。
func (m watchModule) Register(reg module.Registry) error {
	if err := module.Config[watchConfig](reg, "watch"); err != nil {
		return err
	}
	return module.Provide(reg, func(cfg watchConfig) *watchComponent { m.component.value = cfg.Value; return m.component })
}

// TestFileWatchReloadsRunningComponent 使用真实临时文件事件验证监听、去抖、候选重建和
// 组件 Reload 的完整运行链，并由 Context 驱动优雅退出。
func TestFileWatchReloadsRunningComponent(t *testing.T) {
	// 初始文件为 old，Build 构造组件并 Run 建立真实 fsnotify 监听。
	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte("watch:\n  value: old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileSource, err := configsource.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	component := &watchComponent{started: make(chan struct{}), changed: make(chan string, 1)}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(watchModule{component}), app.WithConfigSources(fileSource), app.WithConfigWatch(), app.WithReloadDebounce(50*time.Millisecond), app.WithShutdownTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	// started 通道保证写文件时 Runner 和 Watcher 已经就绪。
	select {
	case <-component.started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	// 覆盖同一文件触发去抖后的全量候选重建。
	if err := os.WriteFile(path, []byte("watch:\n  value: new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	// changed 证明组件 Reload 收到并应用了 new。
	case value := <-component.changed:
		if value != "new" {
			t.Fatalf("reloaded value=%q", value)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("configuration was not reloaded")
	}
	// 取消后等待正常关闭，防止 watcher 或 Runner goroutine 泄漏。
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

// restartWatchComponent 依赖 watchConfig 但故意不实现 Reloader。
type restartWatchComponent struct {
	// started 同步 Runner 已进入。
	started chan struct{}
}

// Run 通知启动并等待取消。
func (c *restartWatchComponent) Run(ctx context.Context) error {
	close(c.started)
	<-ctx.Done()
	return ctx.Err()
}

// restartWatchModule 声明配置和不支持 Reload 的组件。
type restartWatchModule struct {
	// component 是测试预建实例。
	component *restartWatchComponent
}

// Name 返回重启测试模块名。
func (restartWatchModule) Name() string { return "restart-watch" }

// Register 声明 watchConfig 并提供不实现 Reloader 的组件。
func (m restartWatchModule) Register(reg module.Registry) error {
	if err := module.Config[watchConfig](reg, "watch"); err != nil {
		return err
	}
	return module.Provide(reg, func(watchConfig) *restartWatchComponent { return m.component })
}

// TestFileWatchRequestsRestartForComponentWithoutReloader 验证配置变化保守请求进程重启。
func TestFileWatchRequestsRestartForComponentWithoutReloader(t *testing.T) {
	// 初始配置完成 Build 和 Runner 启动。
	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte("watch:\n  value: old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileSource, err := configsource.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	component := &restartWatchComponent{started: make(chan struct{})}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(restartWatchModule{component}), app.WithConfigSources(fileSource), app.WithConfigWatch(), app.WithReloadDebounce(50*time.Millisecond), app.WithReloadTimeout(time.Second), app.WithShutdownTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	select {
	case <-component.started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	// 组件直接依赖变化配置却无 Reloader，Runtime 不得混用新旧状态。
	if err := os.WriteFile(path, []byte("watch:\n  value: new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	// Run 错误和可查询状态都必须表达 RestartRequired。
	case err := <-done:
		if !errors.Is(err, kernelapp.ErrRestartRequired) {
			t.Fatalf("Run() error=%v", err)
		}
		if application.State() != kernelapp.RestartRequired {
			t.Fatalf("State()=%s", application.State())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("application did not request restart")
	}
}
