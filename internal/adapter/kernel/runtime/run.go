package runtime

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiled"
	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/diagnostic"
	"github.com/rin721/micro-go/internal/kernel/lifecycle"
	reloadcontract "github.com/rin721/micro-go/internal/kernel/reload"
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
		return kernelapp.ErrAlreadyRun
	}
	if a.State() != kernelapp.Built {
		return fmt.Errorf("cannot run application in state %s", a.State())
	}

	// Prepare 与 Start 共享启动预算，防止每个组件各自获得完整超时而无限放大总启动时间。
	startupCtx, cancelStartup := context.WithTimeout(ctx, a.options.startupTimeout)
	defer cancelStartup()
	if err := a.setState(kernelapp.Preparing, diagnostic.Prepare); err != nil {
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

	if err := a.setState(kernelapp.Starting, diagnostic.Start); err != nil {
		return a.failAndClose(ctx, nil, err)
	}
	// started 精确记录成功 Start 的实例；启动中途失败时只 Stop 这些实例，但仍 Close 全部实例。
	started := make([]bool, len(a.instances))
	for index, instance := range a.instances {
		component, ok := instance.Value.(lifecycle.Starter)
		if !ok {
			continue
		}
		callErr := guarded(func() error { return component.Start(startupCtx) })
		if callErr == nil {
			started[index] = true
		}
		observeErr := a.emit(componentEvent(instance, diagnostic.Start, callErr))
		if callErr != nil || observeErr != nil {
			var lifecycleErr error
			if callErr != nil {
				lifecycleErr = componentError(instance, diagnostic.Start, callErr)
			}
			return a.failAndClose(ctx, started, errors.Join(lifecycleErr, observeErr))
		}
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
		watchEvents, watchErrors, err = a.runtime.dependencies.Watcher.Watch(runCtx, a.options.sources)
		if err != nil {
			return a.shutdown(cancelRun, started, runnerResults, runnerCount, err, kernelapp.Failed)
		}
	}
	if err := a.setState(kernelapp.Running, diagnostic.Run); err != nil {
		return a.shutdown(cancelRun, started, runnerResults, runnerCount, err, kernelapp.Failed)
	}

	finished := make(map[int]struct{}, runnerCount)
	var primary error
	finalState := kernelapp.Closed
	// Timer 只在收到首个文件事件后创建；连续保存通过安全 Stop/排空/Reset 合并为一次 Reload。
	var debounce *time.Timer
	var debounceChannel <-chan time.Time
	for primary == nil {
		select {
		case <-ctx.Done():
			primary = context.Canceled
		case result := <-runnerResults:
			finished[result.index] = struct{}{}
			observeErr := a.emit(componentEvent(a.instances[result.index], diagnostic.Run, result.err))
			if result.err != nil && !(errors.Is(result.err, context.Canceled) && ctx.Err() != nil) {
				failureObserveErr := a.emit(kernelapp.Event{Kind: kernelapp.RunnerFailed, State: kernelapp.Running, Phase: diagnostic.Run, Err: result.err})
				primary = errors.Join(result.err, observeErr, failureObserveErr)
				finalState = kernelapp.Failed
			} else if observeErr != nil {
				primary = observeErr
				finalState = kernelapp.Failed
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
				finalState = kernelapp.Failed
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
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				primary = context.Canceled
				finalState = kernelapp.Closed
			} else if errors.Is(err, kernelapp.ErrRestartRequired) {
				primary = err
				finalState = kernelapp.RestartRequired
			} else if err != nil {
				primary = err
				finalState = kernelapp.Failed
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
	reloadCtx, cancelReload := context.WithTimeout(ctx, a.options.reloadTimeout)
	defer cancelReload()
	// 每次重载都创建全新的 Loader/Koanf 实例。这样候选失败不会原地污染当前配置，
	// 也规避 Koanf Load/Get 并发访问需要额外同步的问题。
	candidate, err := a.runtime.dependencies.Loader.Load(reloadCtx, a.snapshot.Version+1, a.options.sources, a.plan.Configs)
	if err != nil {
		if errors.Is(reloadCtx.Err(), context.Canceled) {
			return reloadCtx.Err()
		}
		if contextErr := reloadCtx.Err(); contextErr != nil {
			return a.reloadFailure(fmt.Errorf("load reload candidate: %w", contextErr))
		}
		if observeErr := a.emit(kernelapp.Event{Kind: kernelapp.ConfigurationFail, State: kernelapp.Running, Phase: diagnostic.ConfigValidate, Err: err}); observeErr != nil {
			return observeErr
		}
		return nil
	}
	if contextErr := reloadCtx.Err(); contextErr != nil {
		if errors.Is(contextErr, context.Canceled) {
			return contextErr
		}
		return a.reloadFailure(fmt.Errorf("load reload candidate: %w", contextErr))
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
			return a.reloadFailure(kernelapp.ErrRestartRequired)
		}
		var result reloadcontract.Result
		if err := guarded(func() error {
			var callErr error
			result, callErr = reloader.Reload(reloadCtx, candidate.Snapshot)
			return callErr
		}); err != nil {
			if errors.Is(reloadCtx.Err(), context.Canceled) {
				return reloadCtx.Err()
			}
			return a.reloadFailure(componentError(instance, diagnostic.Reload, err))
		}
		if err := reloadCtx.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			return a.reloadFailure(componentError(instance, diagnostic.Reload, err))
		}
		switch result {
		case reloadcontract.Applied, reloadcontract.Ignored:
		case reloadcontract.RestartRequired:
			return a.reloadFailure(kernelapp.ErrRestartRequired)
		default:
			return a.reloadFailure(componentError(instance, diagnostic.Reload, fmt.Errorf("invalid reload result %d", result)))
		}
	}
	// 只有全部受影响组件都返回 Applied 或 Ignored，候选才成为 Application 的当前快照。
	a.snapshot = candidate.Snapshot
	return a.emit(kernelapp.Event{Kind: kernelapp.ConfigurationLoad, State: kernelapp.Running, Phase: diagnostic.ConfigLoad})
}

func (a *Application) reloadFailure(cause error) error {
	observeErr := a.emit(kernelapp.Event{Kind: kernelapp.ConfigurationFail, State: kernelapp.Running, Phase: diagnostic.Reload, Err: cause})
	return errors.Join(cause, observeErr)
}

func (a *Application) forward(ctx context.Context, phase diagnostic.Phase, call func(compiled.Instance) (bool, error)) error {
	// forward 保持编译计划的稳定正序，并在每次调用后检查共享阶段超时。
	for _, instance := range a.instances {
		called, err := call(instance)
		if !called {
			continue
		}
		observeErr := a.emit(componentEvent(instance, phase, err))
		if err != nil || observeErr != nil {
			var lifecycleErr error
			if err != nil {
				lifecycleErr = componentError(instance, phase, err)
			}
			return errors.Join(lifecycleErr, observeErr)
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
	return a.shutdown(func() {}, started, nil, 0, cause, kernelapp.Failed)
}

func (a *Application) shutdown(cancel context.CancelFunc, started []bool, runnerResults <-chan runnerResult, runnersRemaining int, primary error, final kernelapp.State) error {
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
	shutdownFailed := false
	if err := a.setState(kernelapp.Stopping, diagnostic.Stop); err != nil {
		errorsList = append(errorsList, err)
		shutdownFailed = true
	}
	// Stop 只面向成功启动的组件，并按依赖逆序执行。
	for index := len(a.instances) - 1; index >= 0; index-- {
		if started != nil && (index >= len(started) || !started[index]) {
			continue
		}
		stopper, ok := a.instances[index].Value.(lifecycle.Stopper)
		if !ok {
			continue
		}
		callErr := guarded(func() error { return stopper.Stop(shutdownCtx) })
		if callErr == nil {
			if contextErr := shutdownCtx.Err(); contextErr != nil {
				callErr = fmt.Errorf("shutdown Stop timeout: %w", contextErr)
			}
		}
		if callErr != nil {
			errorsList = append(errorsList, componentError(a.instances[index], diagnostic.Stop, callErr))
			shutdownFailed = true
		}
		if err := a.emit(componentEvent(a.instances[index], diagnostic.Stop, callErr)); err != nil {
			errorsList = append(errorsList, err)
			shutdownFailed = true
		}
	}
	// 等待 Runner 确认退出后再 Close，避免后台任务继续访问已释放资源。
	for runnersRemaining > 0 && runnerResults != nil {
		select {
		case result := <-runnerResults:
			runnersRemaining--
			if err := a.emit(componentEvent(a.instances[result.index], diagnostic.Run, result.err)); err != nil {
				errorsList = append(errorsList, err)
				shutdownFailed = true
			}
			if result.err != nil && !errors.Is(result.err, context.Canceled) {
				errorsList = append(errorsList, result.err)
				shutdownFailed = true
			}
		case <-shutdownCtx.Done():
			errorsList = append(errorsList, fmt.Errorf("wait for runners: %w", shutdownCtx.Err()))
			shutdownFailed = true
			runnersRemaining = 0
		}
	}
	if err := a.setState(kernelapp.Closing, diagnostic.Close); err != nil {
		errorsList = append(errorsList, err)
		shutdownFailed = true
	}
	// Close 面向所有构造成功的实例，不受 started 标记限制。
	for index := len(a.instances) - 1; index >= 0; index-- {
		closer, ok := a.instances[index].Value.(lifecycle.Closer)
		if !ok {
			continue
		}
		callErr := guarded(func() error { return closer.Close(shutdownCtx) })
		if callErr == nil {
			if contextErr := shutdownCtx.Err(); contextErr != nil {
				callErr = fmt.Errorf("shutdown Close timeout: %w", contextErr)
			}
		}
		if callErr != nil {
			errorsList = append(errorsList, componentError(a.instances[index], diagnostic.Close, callErr))
			shutdownFailed = true
		}
		if err := a.emit(componentEvent(a.instances[index], diagnostic.Close, callErr)); err != nil {
			errorsList = append(errorsList, err)
			shutdownFailed = true
		}
	}
	if shutdownFailed {
		final = kernelapp.Failed
	}
	a.stateValue.Store(uint32(final))
	if err := a.emit(kernelapp.Event{Kind: kernelapp.StateChanged, State: final}); err != nil {
		errorsList = append(errorsList, err)
		final = kernelapp.Failed
		a.stateValue.Store(uint32(final))
	}
	return errors.Join(errorsList...)
}

func (a *Application) setState(state kernelapp.State, phase diagnostic.Phase) error {
	a.stateValue.Store(uint32(state))
	return a.emit(kernelapp.Event{Kind: kernelapp.StateChanged, State: state, Phase: phase})
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

func componentEvent(instance compiled.Instance, phase diagnostic.Phase, err error) kernelapp.Event {
	return kernelapp.Event{Kind: kernelapp.ComponentEvent, Phase: phase, Module: instance.Provider.Module, Component: instance.Provider.Type.String(), Err: err}
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
