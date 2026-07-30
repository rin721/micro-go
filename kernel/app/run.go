package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/rin721/micro-go/internal/config/koanfadapter"
	"github.com/rin721/micro-go/internal/config/loading"
	configwatcher "github.com/rin721/micro-go/internal/config/watcher"
	"github.com/rin721/micro-go/internal/di/compiled"
	"github.com/rin721/micro-go/kernel/config"
	"github.com/rin721/micro-go/kernel/diagnostic"
	"github.com/rin721/micro-go/kernel/lifecycle"
	reloadcontract "github.com/rin721/micro-go/kernel/reload"
)

type runnerResult struct {
	index int
	err   error
}

// Run 按 Prepare、Start、并发 Run、Stop、Close 的顺序驱动一次完整生命周期。
// 正向阶段遵循依赖顺序，反向阶段遵循消费者优先顺序；任何失败都会汇入同一关闭路径。
func (a *Application) Run(ctx context.Context) error {
	if a == nil {
		return errors.New("application is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// 原子闸门同时阻止串行和并发的第二次 Run，避免重复启动同一批资源。
	if !a.runStarted.CompareAndSwap(false, true) {
		return ErrAlreadyRun
	}
	if a.State() != Built {
		return fmt.Errorf("cannot run application in state %s", a.State())
	}

	// Prepare 与 Start 共享启动预算，防止每个组件各自获得完整超时而无限放大总启动时间。
	startupCtx, cancelStartup := context.WithTimeout(ctx, a.options.startupTimeout)
	defer cancelStartup()
	if err := a.setState(Preparing, diagnostic.Prepare); err != nil {
		return a.failAndClose(ctx, nil, err)
	}
	if err := a.forward(startupCtx, diagnostic.Prepare, func(instance compiled.Instance) (bool, error) {
		component, ok := instance.Value.(lifecycle.Preparer)
		if !ok {
			return false, nil
		}
		return true, guarded(func() error { return component.Prepare(startupCtx) })
	}); err != nil {
		return a.failAndClose(ctx, nil, err)
	}

	if err := a.setState(Starting, diagnostic.Start); err != nil {
		return a.failAndClose(ctx, nil, err)
	}
	// started 精确记录成功 Start 的实例；启动中途失败时只 Stop 这些实例，但仍 Close 全部实例。
	started := make([]bool, len(a.instances))
	for index, instance := range a.instances {
		component, ok := instance.Value.(lifecycle.Starter)
		if !ok {
			continue
		}
		if err := a.emit(componentEvent(instance, diagnostic.Start, nil)); err != nil {
			return a.failAndClose(ctx, started, err)
		}
		if err := guarded(func() error { return component.Start(startupCtx) }); err != nil {
			return a.failAndClose(ctx, started, componentError(instance, diagnostic.Start, err))
		}
		started[index] = true
	}
	if err := startupCtx.Err(); err != nil {
		return a.failAndClose(ctx, started, fmt.Errorf("application startup timeout: %w", err))
	}

	// 所有 Runner 共享一个可取消 Context，任一 Runner 或监听器失败即可通知其他任务退出。
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	runnerResults := make(chan runnerResult, len(a.instances))
	runnerCount := 0
	for index, instance := range a.instances {
		runner, ok := instance.Value.(lifecycle.Runner)
		if !ok {
			continue
		}
		runnerCount++
		// 每个 Runner 独立执行，但只向带缓冲结果通道写一次；缓冲容量保证关停方即使
		// 尚未开始接收，Runner 也不会因汇报结果而泄漏。
		go func(index int, instance compiled.Instance, runner lifecycle.Runner) {
			err := guarded(func() error { return runner.Run(runCtx) })
			if err != nil {
				err = componentError(instance, diagnostic.Run, err)
			}
			runnerResults <- runnerResult{index: index, err: err}
		}(index, instance, runner)
	}

	// nil channel 在 select 中天然禁用对应分支，因此未启用监听时不需要额外 goroutine 或轮询。
	watchEvents, watchErrors := (<-chan config.Change)(nil), (<-chan error)(nil)
	if a.options.watch && hasWatchSource(a.options.sources) {
		var err error
		watchEvents, watchErrors, err = configwatcher.Watch(runCtx, a.options.sources)
		if err != nil {
			return a.shutdown(cancelRun, started, runnerResults, runnerCount, err, Failed)
		}
	}
	if err := a.setState(Running, diagnostic.Run); err != nil {
		return a.shutdown(cancelRun, started, runnerResults, runnerCount, err, Failed)
	}

	finished := make(map[int]struct{}, runnerCount)
	var primary error
	finalState := Closed
	// Timer 只在收到首个文件事件后创建；连续保存通过安全 Stop/排空/Reset 合并为一次 Reload。
	var debounce *time.Timer
	var debounceChannel <-chan time.Time
	for primary == nil {
		select {
		case <-ctx.Done():
			primary = context.Canceled
		case result := <-runnerResults:
			finished[result.index] = struct{}{}
			if result.err != nil {
				_ = a.emit(Event{Kind: RunnerFailed, State: Running, Phase: diagnostic.Run, Err: result.err})
				primary = result.err
				finalState = Failed
			} else {
				// Runner 正常提前返回同样意味着长期任务结束，应用不能假装仍在正常运行。
				primary = context.Canceled
			}
		case err, ok := <-watchErrors:
			if !ok {
				watchErrors = nil
				continue
			}
			if err != nil {
				primary = err
				finalState = Failed
			}
		case _, ok := <-watchEvents:
			if !ok {
				watchEvents = nil
				continue
			}
			if debounce == nil {
				debounce = time.NewTimer(a.options.reloadDebounce)
			} else {
				if !debounce.Stop() {
					select {
					case <-debounce.C:
					default:
					}
				}
				debounce.Reset(a.options.reloadDebounce)
			}
			debounceChannel = debounce.C
		case <-debounceChannel:
			debounceChannel = nil
			if ctx.Err() != nil {
				primary = context.Canceled
				continue
			}
			// 关停信号优先于生成新候选，避免 Shutdown 与 Reload 同时修改组件状态。
			err := a.reload(runCtx)
			if errors.Is(err, ErrRestartRequired) {
				primary = err
				finalState = RestartRequired
			} else if err != nil {
				primary = err
				finalState = Failed
			}
		}
	}
	if debounce != nil {
		debounce.Stop()
	}
	if errors.Is(primary, context.Canceled) {
		primary = nil
	}
	return a.shutdown(cancelRun, started, runnerResults, runnerCount-len(finished), primary, finalState)
}

func (a *Application) reload(ctx context.Context) error {
	// 每次重载都创建全新的 Loader/Koanf 实例。这样候选失败不会原地污染当前配置，
	// 也规避 Koanf Load/Get 并发访问需要额外同步的问题。
	var loader loading.Loader = koanfadapter.New()
	candidate, err := loader.Load(ctx, a.snapshot.Version+1, a.options.sources, a.plan.Configs)
	if err != nil {
		if observeErr := a.emit(Event{Kind: ConfigurationFail, State: Running, Phase: diagnostic.ConfigValidate, Err: err}); observeErr != nil {
			return observeErr
		}
		return nil
	}
	// 使用规范化 JSON 摘要按配置类型比较，只通知真正依赖变化配置的组件。
	changed := make(map[reflect.Type]struct{})
	for _, declaration := range a.plan.Configs {
		oldHash, oldOK := a.snapshot.Hash(declaration.Type)
		newHash, newOK := candidate.Snapshot.Hash(declaration.Type)
		if !oldOK || !newOK || oldHash != newHash {
			changed[declaration.Type] = struct{}{}
		}
	}
	for _, instance := range a.instances {
		affected := false
		for _, dependency := range instance.Provider.Dependencies {
			if dependency.Config {
				_, affected = changed[dependency.Resolved]
				if affected {
					break
				}
			}
		}
		if !affected {
			continue
		}
		reloader, ok := instance.Value.(reloadcontract.Reloader)
		if !ok {
			// 组件无法证明可安全原地更新时，宁可请求重启，也不让新旧配置混用。
			return ErrRestartRequired
		}
		var result reloadcontract.Result
		if err := guarded(func() error {
			var callErr error
			result, callErr = reloader.Reload(ctx, candidate.Snapshot)
			return callErr
		}); err != nil {
			return componentError(instance, diagnostic.ConfigValidate, err)
		}
		if result == reloadcontract.RestartRequired {
			return ErrRestartRequired
		}
	}
	// 只有全部受影响组件都返回 Applied 或 Ignored，候选才成为 Application 的当前快照。
	a.snapshot = candidate.Snapshot
	return a.emit(Event{Kind: ConfigurationLoad, State: Running, Phase: diagnostic.ConfigLoad})
}

func (a *Application) forward(ctx context.Context, phase diagnostic.Phase, call func(compiled.Instance) (bool, error)) error {
	// forward 保持编译计划的稳定正序，并在每次调用后检查共享阶段超时。
	for _, instance := range a.instances {
		called, err := call(instance)
		if !called {
			continue
		}
		if observeErr := a.emit(componentEvent(instance, phase, err)); observeErr != nil {
			return observeErr
		}
		if err != nil {
			return componentError(instance, phase, err)
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("application %s timeout: %w", phase, err)
		}
	}
	return nil
}

func (a *Application) failAndClose(parent context.Context, started []bool, cause error) error {
	// 即使启动前失败也复用 shutdown，确保所有已构造 Closer 恰好进入统一释放路径。
	if started == nil {
		started = make([]bool, len(a.instances))
	}
	return a.shutdown(func() {}, started, nil, 0, cause, Failed)
}

func (a *Application) shutdown(cancel context.CancelFunc, started []bool, runnerResults <-chan runnerResult, runnersRemaining int, primary error, final State) error {
	// 先取消 Runner，再使用独立后台 Context 关闭资源；父 Context 往往已经取消，直接复用
	// 会让 Stop/Close 没有执行清理的时间窗口。
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), a.options.shutdownTimeout)
	defer shutdownCancel()
	// 保留首要故障，同时聚合 Stop、Runner 等待和 Close 错误，避免后续清理故障被覆盖。
	errorsList := make([]error, 0)
	if primary != nil {
		errorsList = append(errorsList, primary)
	}
	_ = a.setState(Stopping, diagnostic.Stop)
	// Stop 只面向成功启动的组件，并按依赖逆序执行。
	for index := len(a.instances) - 1; index >= 0; index-- {
		if started != nil && (index >= len(started) || !started[index]) {
			continue
		}
		stopper, ok := a.instances[index].Value.(lifecycle.Stopper)
		if !ok {
			continue
		}
		if err := guarded(func() error { return stopper.Stop(shutdownCtx) }); err != nil {
			errorsList = append(errorsList, componentError(a.instances[index], diagnostic.Stop, err))
		}
	}
	// 等待 Runner 确认退出后再 Close，避免后台任务继续访问已释放资源。
	for runnersRemaining > 0 && runnerResults != nil {
		select {
		case result := <-runnerResults:
			runnersRemaining--
			if result.err != nil && !errors.Is(result.err, context.Canceled) {
				errorsList = append(errorsList, result.err)
			}
		case <-shutdownCtx.Done():
			errorsList = append(errorsList, fmt.Errorf("wait for runners: %w", shutdownCtx.Err()))
			runnersRemaining = 0
		}
	}
	_ = a.setState(Closing, diagnostic.Close)
	// Close 面向所有构造成功的实例，不受 started 标记限制。
	for index := len(a.instances) - 1; index >= 0; index-- {
		closer, ok := a.instances[index].Value.(lifecycle.Closer)
		if !ok {
			continue
		}
		if err := guarded(func() error { return closer.Close(shutdownCtx) }); err != nil {
			errorsList = append(errorsList, componentError(a.instances[index], diagnostic.Close, err))
		}
	}
	if len(errorsList) > 0 && final == Closed {
		final = Failed
	}
	a.stateValue.Store(uint32(final))
	_ = a.emit(Event{Kind: StateChanged, State: final})
	return errors.Join(errorsList...)
}

func (a *Application) setState(state State, phase diagnostic.Phase) error {
	a.stateValue.Store(uint32(state))
	return a.emit(Event{Kind: StateChanged, State: state, Phase: phase})
}

func guarded(call func() error) (err error) {
	// 生命周期方法属于用户代码边界，panic 必须转换为带堆栈的项目错误并进入回滚流程。
	defer func() {
		if value := recover(); value != nil {
			err = diagnostic.NewPanicError(value)
		}
	}()
	return call()
}

func componentEvent(instance compiled.Instance, phase diagnostic.Phase, err error) Event {
	return Event{Kind: ComponentEvent, Phase: phase, Module: instance.Provider.Module, Component: instance.Provider.Type.String(), Err: err}
}

func componentError(instance compiled.Instance, phase diagnostic.Phase, err error) error {
	return &diagnostic.ComponentError{Module: instance.Provider.Module, Component: instance.Provider.Type.String(), Provider: instance.Provider.Name, Phase: phase, Cause: err}
}

func hasWatchSource(sources []config.Source) bool {
	for _, source := range sources {
		if _, ok := source.(config.WatchSource); ok {
			return true
		}
	}
	return false
}
