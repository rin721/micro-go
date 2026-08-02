// 本文件通过故障注入验证各生命周期阶段的错误、panic、超时和观察失败都进入统一清理。
package runtime_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	app "github.com/rin721/micro-go/internal/adapter/kernel/runtime"
	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
	"github.com/rin721/micro-go/internal/kernel/diagnostic"
	"github.com/rin721/micro-go/internal/kernel/module"
	"github.com/rin721/micro-go/internal/kernel/testkit"
)

// failureComponent 允许每个生命周期方法独立注入行为，并记录最终是否 Close。
type failureComponent struct {
	// prepare 覆盖 Prepare 的场景行为。
	prepare func(context.Context) error
	// start 覆盖 Start 的场景行为。
	start func(context.Context) error
	// run 覆盖 Run 的场景行为。
	run func(context.Context) error
	// stop 覆盖 Stop 的场景行为。
	stop func(context.Context) error
	// close 覆盖 Close 的场景行为。
	close func(context.Context) error
	// closed 标记 Close 至少进入一次。
	closed bool
}

// Prepare 调用可选注入函数，未设置时成功。
func (c *failureComponent) Prepare(ctx context.Context) error {
	if c.prepare != nil {
		return c.prepare(ctx)
	}
	return nil
}

// Start 调用可选注入函数，未设置时成功。
func (c *failureComponent) Start(ctx context.Context) error {
	if c.start != nil {
		return c.start(ctx)
	}
	return nil
}

// Run 调用可选注入函数，未设置时会意外正常返回。
func (c *failureComponent) Run(ctx context.Context) error {
	if c.run != nil {
		return c.run(ctx)
	}
	return nil
}

// Stop 调用可选注入函数，未设置时成功。
func (c *failureComponent) Stop(ctx context.Context) error {
	if c.stop != nil {
		return c.stop(ctx)
	}
	return nil
}

// Close 先标记已进入，再调用可选注入函数。
func (c *failureComponent) Close(ctx context.Context) error {
	c.closed = true
	if c.close != nil {
		return c.close(ctx)
	}
	return nil
}

// failureModule 把预建 failureComponent 作为唯一 Provider 放入 Runtime。
type failureModule struct {
	// component 是测试控制和观察的同一实例。
	component *failureComponent
}

// Name 返回故障注入模块名。
func (failureModule) Name() string { return "failure" }

// Register 用闭包返回预建组件，避免构造逻辑干扰生命周期场景。
func (m failureModule) Register(registry module.Registry) error {
	return module.Provide(registry, func() *failureComponent { return m.component })
}

// runUntilRootCancellation 等待 Runner 确认启动后取消根 Context，并有界等待 Run 返回。
func runUntilRootCancellation(t *testing.T, application *app.Application, component *failureComponent) error {
	t.Helper()
	started := make(chan struct{})
	// 覆盖 Runner 行为：先同步启动事实，再协作等待取消。
	component.run = func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	// done 带缓冲，测试超时返回时 Runner 汇报不会继续阻塞 goroutine。
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	select {
	case <-started:
		cancel()
	case err := <-done:
		cancel()
		return err
	case <-time.After(time.Second):
		// 启动超时也取消 Context，防止被测 goroutine 遗留。
		cancel()
		t.Fatal("runner did not start")
	}
	// 第二个有界等待验证 Application 完成 Stop、Runner 汇报和 Close。
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		t.Fatal("application did not stop after cancellation")
		return nil
	}
}

// TestPrepareFailureAndPanicStillCloseComponent 表驱动验证 Prepare error/panic 都释放构造资源。
func TestPrepareFailureAndPanicStillCloseComponent(t *testing.T) {
	tests := []struct {
		// name 标识 error 或 panic 子场景。
		name string
		// prepare 是注入 Prepare 的行为。
		prepare func(context.Context) error
		// message 是期望错误链包含的稳定片段。
		message string
	}{
		{name: "error", prepare: func(context.Context) error { return errors.New("prepare failed") }, message: "prepare failed"},
		{name: "panic", prepare: func(context.Context) error { panic("prepare panic") }, message: "panic: prepare panic"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			component := &failureComponent{prepare: test.prepare}
			application, err := newRuntime(t).Build(context.Background(), app.WithModules(failureModule{component}), app.WithShutdownTimeout(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			err = application.Run(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.message) || !component.closed {
				t.Fatalf("Run() error=%v closed=%v", err, component.closed)
			}
			if application.State() != kernelapp.Failed {
				t.Fatalf("State()=%s", application.State())
			}
		})
	}
}

// TestRunnerStopAndCloseErrorsAreAllPreserved 验证主错误和两项清理错误均可 errors.Is。
func TestRunnerStopAndCloseErrorsAreAllPreserved(t *testing.T) {
	// 三个独立哨兵分别来自 Run、Stop 和 Close。
	runnerErr := errors.New("runner failed")
	stopErr := errors.New("stop failed")
	closeErr := errors.New("close failed")
	component := &failureComponent{
		run:   func(context.Context) error { return runnerErr },
		stop:  func(context.Context) error { return stopErr },
		close: func(context.Context) error { return closeErr },
	}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(failureModule{component}), app.WithShutdownTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	err = application.Run(context.Background())
	// errors.Join 必须保留每个哨兵，而不是只返回最后一项。
	for _, target := range []error{runnerErr, stopErr, closeErr} {
		if !errors.Is(err, target) {
			t.Fatalf("Run() error %v does not preserve %v", err, target)
		}
	}
	if application.State() != kernelapp.Failed {
		t.Fatalf("State()=%s", application.State())
	}
}

// TestRunnerUnexpectedNormalReturnFailsApplication 锁定长期 Runner 返回 nil 的失败语义。
func TestRunnerUnexpectedNormalReturnFailsApplication(t *testing.T) {
	component := &failureComponent{run: func(context.Context) error { return nil }}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(failureModule{component}), app.WithShutdownTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	err = application.Run(context.Background())
	if !errors.Is(err, kernelapp.ErrRunnerExited) {
		t.Fatalf("Run() error=%v, want ErrRunnerExited", err)
	}
	if !component.closed || application.State() != kernelapp.Failed {
		t.Fatalf("closed=%v state=%s", component.closed, application.State())
	}
}

// TestRunnerStopAndClosePanicsAreAllPreserved 验证三个用户边界 panic 均被捕获且不跳过清理。
func TestRunnerStopAndClosePanicsAreAllPreserved(t *testing.T) {
	component := &failureComponent{
		run:   func(context.Context) error { panic("runner panic") },
		stop:  func(context.Context) error { panic("stop panic") },
		close: func(context.Context) error { panic("close panic") },
	}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(failureModule{component}), app.WithShutdownTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	err = application.Run(context.Background())
	// 每个 panic 文本都必须保留在聚合错误中。
	for _, message := range []string{"runner panic", "stop panic", "close panic"} {
		if err == nil || !strings.Contains(err.Error(), message) {
			t.Fatalf("Run() error %v does not contain %q", err, message)
		}
	}
	if !component.closed || application.State() != kernelapp.Failed {
		t.Fatalf("closed=%v state=%s", component.closed, application.State())
	}
}

// TestStartPanicStillClosesComponent 验证 Start panic 不会越过 Runtime 或跳过 Close。
func TestStartPanicStillClosesComponent(t *testing.T) {
	component := &failureComponent{start: func(context.Context) error { panic("start panic") }}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(failureModule{component}), app.WithShutdownTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	err = application.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "start panic") || !component.closed {
		t.Fatalf("Run() error=%v closed=%v", err, component.closed)
	}
}

// TestStartupTimeoutIsCooperativeAndClosesComponent 验证 Prepare 响应共享超时后仍进入 Close。
func TestStartupTimeoutIsCooperativeAndClosesComponent(t *testing.T) {
	// Prepare 阻塞到 Context 完成并返回其原因，模拟正确协作组件。
	component := &failureComponent{prepare: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(failureModule{component}), app.WithStartupTimeout(20*time.Millisecond), app.WithShutdownTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	err = application.Run(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) || !component.closed {
		t.Fatalf("Run() error=%v closed=%v", err, component.closed)
	}
}

// TestShutdownTimeoutIsReportedWhenComponentReturnsAfterDeadline 防止组件超时后返回 nil 掩盖预算耗尽。
func TestShutdownTimeoutIsReportedWhenComponentReturnsAfterDeadline(t *testing.T) {
	// Stop 等到 deadline 后故意返回 nil，Runtime 必须在调用后再次检查 Context。
	component := &failureComponent{stop: func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(failureModule{component}), app.WithShutdownTimeout(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	err = application.Run(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) || !component.closed {
		t.Fatalf("Run() error=%v closed=%v", err, component.closed)
	}
	if application.State() != kernelapp.Failed {
		t.Fatalf("State()=%s", application.State())
	}
}

// shutdownPanicObserver 只在进入 Stopping 状态时 panic。
type shutdownPanicObserver struct{}

// Observe 针对 Stopping 状态注入诊断失败。
func (shutdownPanicObserver) Observe(event kernelapp.Event) {
	if event.Kind == kernelapp.StateChanged && event.State == kernelapp.Stopping {
		panic("shutdown observer panic")
	}
}

// targetedPanicObserver 可选择事件种类和可选状态注入 panic。
type targetedPanicObserver struct {
	// kind 是目标事件种类。
	kind kernelapp.EventKind
	// state 是目标状态；Created 表示不限制状态。
	state kernelapp.State
}

// Observe 在匹配目标事件时触发 panic。
func (o targetedPanicObserver) Observe(event kernelapp.Event) {
	if event.Kind == o.kind && (o.state == kernelapp.Created || event.State == o.state) {
		panic("target observer panic")
	}
}

// TestRuntimeObserverPanicsAreAggregatedAcrossRunAndShutdown 表驱动覆盖运行和关停各状态的观察失败。
func TestRuntimeObserverPanicsAreAggregatedAcrossRunAndShutdown(t *testing.T) {
	tests := []struct {
		// name 标识目标阶段。
		name string
		// kind 是触发 panic 的事件类型。
		kind kernelapp.EventKind
		// state 是触发 panic 的状态。
		state kernelapp.State
		// runnerErr 非 nil 时由 Runner 主动触发失败关停。
		runnerErr error
	}{
		{name: "running", kind: kernelapp.StateChanged, state: kernelapp.Running},
		{name: "runner failed", kind: kernelapp.RunnerFailed, state: kernelapp.Running, runnerErr: errors.New("runner failed")},
		{name: "stopping", kind: kernelapp.StateChanged, state: kernelapp.Stopping},
		{name: "closing", kind: kernelapp.StateChanged, state: kernelapp.Closing},
		{name: "final", kind: kernelapp.StateChanged, state: kernelapp.Closed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			component := &failureComponent{}
			if test.runnerErr != nil {
				component.run = func(context.Context) error { return test.runnerErr }
			}
			observer := targetedPanicObserver{kind: test.kind, state: test.state}
			application, err := newRuntime(t).Build(context.Background(), app.WithModules(failureModule{component}), app.WithObserver(observer), app.WithShutdownTimeout(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			if test.runnerErr == nil {
				// 无 Runner 错误的场景通过根取消进入正常关停路径。
				err = runUntilRootCancellation(t, application, component)
			} else {
				err = application.Run(context.Background())
			}
			if err == nil || !strings.Contains(err.Error(), "target observer panic") || !component.closed {
				t.Fatalf("Run() error=%v closed=%v", err, component.closed)
			}
			if application.State() != kernelapp.Failed {
				t.Fatalf("State()=%s", application.State())
			}
		})
	}
}

// TestShutdownObserverPanicIsReturnedWithoutSkippingClose 专门验证 Stopping 事件失败后仍释放资源。
func TestShutdownObserverPanicIsReturnedWithoutSkippingClose(t *testing.T) {
	component := &failureComponent{}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(failureModule{component}), app.WithObserver(shutdownPanicObserver{}), app.WithShutdownTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	err = runUntilRootCancellation(t, application, component)
	if err == nil || !strings.Contains(err.Error(), "shutdown observer panic") || !component.closed {
		t.Fatalf("Run() error=%v closed=%v", err, component.closed)
	}
	if application.State() != kernelapp.Failed {
		t.Fatalf("State()=%s", application.State())
	}
}

// TestApplicationRejectsSecondRun 验证一次性实例正常关闭后仍拒绝再次启动。
func TestApplicationRejectsSecondRun(t *testing.T) {
	component := &failureComponent{}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(failureModule{component}), app.WithShutdownTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := runUntilRootCancellation(t, application, component); err != nil {
		t.Fatal(err)
	}
	if err := application.Run(context.Background()); !errors.Is(err, kernelapp.ErrAlreadyRun) {
		t.Fatalf("second Run() error=%v", err)
	}
}

// TestRuntimeRejectsNonPositiveReloadTimeout 确保非法时间 Option 在任何构造前失败。
func TestRuntimeRejectsNonPositiveReloadTimeout(t *testing.T) {
	_, err := newRuntime(t).Compile(app.WithReloadTimeout(0))
	if err == nil || !strings.Contains(err.Error(), "reload timeout must be positive") {
		t.Fatalf("Compile() error=%v", err)
	}
}

// TestObserverReceivesEveryLifecycleResult 验证每次真实生命周期调用都有对应 ComponentEvent。
func TestObserverReceivesEveryLifecycleResult(t *testing.T) {
	component := &failureComponent{}
	observer := &testkit.RecorderObserver{}
	application, err := newRuntime(t).Build(context.Background(), app.WithModules(failureModule{component}), app.WithObserver(observer), app.WithShutdownTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := runUntilRootCancellation(t, application, component); err != nil {
		t.Fatal(err)
	}
	var phases []diagnostic.Phase
	// 只收集组件事件，忽略同一运行中的状态和配置事件。
	for _, event := range observer.Events() {
		if event.Kind == kernelapp.ComponentEvent {
			phases = append(phases, event.Phase)
		}
	}
	// 关闭先取消 Runner，再执行 Stop，最后等待 Runner 报告并 Close；因此协作退出时
	// Stop 事件先于 Run 的最终结果，这是运行时声明的资源关闭顺序。
	want := []diagnostic.Phase{diagnostic.Prepare, diagnostic.Start, diagnostic.Stop, diagnostic.Run, diagnostic.Close}
	if len(phases) != len(want) {
		t.Fatalf("component phases=%v", phases)
	}
	for index := range want {
		if phases[index] != want[index] {
			t.Fatalf("component phases=%v want=%v", phases, want)
		}
	}
}
