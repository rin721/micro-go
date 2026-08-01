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

type failureComponent struct {
	prepare func(context.Context) error
	start   func(context.Context) error
	run     func(context.Context) error
	stop    func(context.Context) error
	close   func(context.Context) error
	closed  bool
}

func (c *failureComponent) Prepare(ctx context.Context) error {
	if c.prepare != nil {
		return c.prepare(ctx)
	}
	return nil
}
func (c *failureComponent) Start(ctx context.Context) error {
	if c.start != nil {
		return c.start(ctx)
	}
	return nil
}
func (c *failureComponent) Run(ctx context.Context) error {
	if c.run != nil {
		return c.run(ctx)
	}
	return nil
}
func (c *failureComponent) Stop(ctx context.Context) error {
	if c.stop != nil {
		return c.stop(ctx)
	}
	return nil
}
func (c *failureComponent) Close(ctx context.Context) error {
	c.closed = true
	if c.close != nil {
		return c.close(ctx)
	}
	return nil
}

type failureModule struct{ component *failureComponent }

func (failureModule) Name() string { return "failure" }
func (m failureModule) Register(registry module.Registry) error {
	return module.Provide(registry, func() *failureComponent { return m.component })
}

func runUntilRootCancellation(t *testing.T, application *app.Application, component *failureComponent) error {
	t.Helper()
	started := make(chan struct{})
	component.run = func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
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
		cancel()
		t.Fatal("runner did not start")
	}
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		t.Fatal("application did not stop after cancellation")
		return nil
	}
}

func TestPrepareFailureAndPanicStillCloseComponent(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(context.Context) error
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

func TestRunnerStopAndCloseErrorsAreAllPreserved(t *testing.T) {
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
	for _, target := range []error{runnerErr, stopErr, closeErr} {
		if !errors.Is(err, target) {
			t.Fatalf("Run() error %v does not preserve %v", err, target)
		}
	}
	if application.State() != kernelapp.Failed {
		t.Fatalf("State()=%s", application.State())
	}
}

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
	for _, message := range []string{"runner panic", "stop panic", "close panic"} {
		if err == nil || !strings.Contains(err.Error(), message) {
			t.Fatalf("Run() error %v does not contain %q", err, message)
		}
	}
	if !component.closed || application.State() != kernelapp.Failed {
		t.Fatalf("closed=%v state=%s", component.closed, application.State())
	}
}

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

func TestStartupTimeoutIsCooperativeAndClosesComponent(t *testing.T) {
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

func TestShutdownTimeoutIsReportedWhenComponentReturnsAfterDeadline(t *testing.T) {
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

type shutdownPanicObserver struct{}

func (shutdownPanicObserver) Observe(event kernelapp.Event) {
	if event.Kind == kernelapp.StateChanged && event.State == kernelapp.Stopping {
		panic("shutdown observer panic")
	}
}

type targetedPanicObserver struct {
	kind  kernelapp.EventKind
	state kernelapp.State
}

func (o targetedPanicObserver) Observe(event kernelapp.Event) {
	if event.Kind == o.kind && (o.state == kernelapp.Created || event.State == o.state) {
		panic("target observer panic")
	}
}

func TestRuntimeObserverPanicsAreAggregatedAcrossRunAndShutdown(t *testing.T) {
	tests := []struct {
		name      string
		kind      kernelapp.EventKind
		state     kernelapp.State
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

func TestRuntimeRejectsNonPositiveReloadTimeout(t *testing.T) {
	_, err := newRuntime(t).Compile(app.WithReloadTimeout(0))
	if err == nil || !strings.Contains(err.Error(), "reload timeout must be positive") {
		t.Fatalf("Compile() error=%v", err)
	}
}

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
