// 本文件驱动 Application 生命周期、Runner 监督、配置去抖重载和统一逆序关停。
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

// runnerResult 把并发 Runner 的实例位置与规范化返回错误配对。
type runnerResult struct {
	// index 对应 Application.instances 中的稳定位置。
	index int
	// err 是 nil、Context 取消或已经补充组件上下文的运行错误。
	err error
}

// Run 按 Prepare、Start、并发 Run、Stop、Close 的顺序驱动一次完整生命周期。
// 正向阶段遵循依赖顺序，反向阶段遵循消费者优先顺序；任何失败都会汇入同一关闭路径。
func (a *Application) Run(ctx context.Context) error {
	// nil receiver 无法拥有生命周期，返回普通错误而不是解引用 panic。
	if a == nil {
		return errors.New("application is nil")
	}
	// nil Context 转换为 Background，使取消语义明确为“调用方不主动取消”。
	if ctx == nil {
		ctx = context.Background()
	}
	// 原子闸门同时阻止串行和并发的第二次 Run，避免重复启动同一批资源。
	if !a.runStarted.CompareAndSwap(false, true) {
		return kernelapp.ErrAlreadyRun
	}
	// 只有 Build 完整成功的实例集合可以进入生命周期；其他状态表明内部调用顺序错误。
	if a.State() != kernelapp.Built {
		return fmt.Errorf("cannot run application in state %s", a.State())
	}

	// Prepare 与 Start 共享启动预算，防止每个组件各自获得完整超时而无限放大总启动时间。
	startupCtx, cancelStartup := context.WithTimeout(ctx, a.options.startupTimeout)
	// 即使启动提前返回也释放 timer 资源。
	defer cancelStartup()
	// 状态事件必须先成功发布，再开始调用用户 Prepare。
	if err := a.setState(kernelapp.Preparing, diagnostic.Prepare); err != nil {
		return a.failAndClose(ctx, nil, err)
	}
	// forward 跳过未实现 Preparer 的实例，并对每次真实调用发布组件事件。
	if err := a.forward(startupCtx, diagnostic.Prepare, func(instance compiled.Instance) (bool, error) {
		component, ok := instance.Value.(lifecycle.Preparer)
		if !ok {
			return false, nil
		}
		return true, guarded(func() error { return component.Prepare(startupCtx) })
	}); err != nil {
		return a.failAndClose(ctx, nil, err)
	}

	// Prepare 全部成功后才进入 Starting，保证组件不会在依赖未准备时启动。
	if err := a.setState(kernelapp.Starting, diagnostic.Start); err != nil {
		return a.failAndClose(ctx, nil, err)
	}
	// started 精确记录成功 Start 的实例；启动中途失败时只 Stop 这些实例，但仍 Close 全部实例。
	started := make([]bool, len(a.instances))
	// Start 按依赖正序逐个执行并立即记录成功位。
	for index, instance := range a.instances {
		component, ok := instance.Value.(lifecycle.Starter)
		if !ok {
			continue
		}
		// guarded 把组件 panic 转为普通错误，仍能进入统一回滚。
		callErr := guarded(func() error { return component.Start(startupCtx) })
		if callErr == nil {
			started[index] = true
		}
		// 无论调用结果如何都发布一次组件事件，观察失败与业务失败一并处理。
		observeErr := a.emit(componentEvent(instance, diagnostic.Start, callErr))
		if callErr != nil || observeErr != nil {
			var lifecycleErr error
			if callErr != nil {
				lifecycleErr = componentError(instance, diagnostic.Start, callErr)
			}
			// started 切片确保失败点之前成功启动的组件会被 Stop。
			return a.failAndClose(ctx, started, errors.Join(lifecycleErr, observeErr))
		}
	}
	// 即使最后一个 Start 返回 nil，共享预算也可能刚好到期，因此阶段末再次检查。
	if err := startupCtx.Err(); err != nil {
		return a.failAndClose(ctx, started, fmt.Errorf("application startup timeout: %w", err))
	}

	// 运行 Context 同时拥有 Runner 与配置监听器。Watcher 必须先于 Runner 建立，随后立即
	// 重读一次配置：这样 Build 初次加载到 Run 建立监听之间发生的变化不会永久丢失，
	// Runner 对外暴露“已启动”时也已经不存在尚未监听的竞态窗口。
	runCtx, cancelRun := context.WithCancel(ctx)
	// 所有退出路径都至少取消一次；CancelFunc 幂等，shutdown 可提前调用同一个函数。
	defer cancelRun()
	// nil 通道用于在 select 中禁用尚未启用的监听分支。
	watchEvents, watchErrors := (<-chan config.Change)(nil), (<-chan error)(nil)
	if a.options.watch && hasWatchSource(a.options.sources) {
		// Watcher 创建失败发生在 Runner 启动前，直接关停已启动组件。
		var err error
		watchEvents, watchErrors, err = a.runtime.dependencies.Watcher.Watch(runCtx, a.options.sources)
		if err != nil {
			return a.shutdown(cancelRun, started, nil, 0, err, kernelapp.Failed)
		}
		// 建立监听后立即重载一次，封闭 Build 与 Watch 注册之间的变化窗口。
		if err := a.reload(runCtx); err != nil {
			// 组件明确要求重启时保留 RestartRequired 终态，其他错误进入 Failed。
			final := kernelapp.Failed
			if errors.Is(err, kernelapp.ErrRestartRequired) {
				final = kernelapp.RestartRequired
			}
			return a.shutdown(cancelRun, started, nil, 0, err, final)
		}
	}

	// 所有 Runner 共享一个可取消 Context，任一 Runner 或监听器失败即可通知其他任务退出。
	runnerResults := make(chan runnerResult, len(a.instances))
	runnerCount := 0
	// 实例按稳定依赖顺序扫描，结果中的 index 可直接用于事件和关停定位。
	for index, instance := range a.instances {
		runner, ok := instance.Value.(lifecycle.Runner)
		if !ok {
			continue
		}
		runnerCount++
		// 每个 Runner 独立执行，但只向带缓冲结果通道写一次；缓冲容量保证关停方即使
		// 尚未开始接收，Runner 也不会因汇报结果而泄漏。
		go func(index int, instance compiled.Instance, runner lifecycle.Runner) {
			// Runner panic 和普通错误先规范化，再附加组件运行阶段上下文。
			err := guarded(func() error { return runner.Run(runCtx) })
			if err != nil {
				err = componentError(instance, diagnostic.Run, err)
			}
			// 每个 goroutine 只发送一次；通道容量按实例数足以容纳全部最终结果。
			runnerResults <- runnerResult{index: index, err: err}
		}(index, instance, runner)
	}

	// nil channel 在 select 中天然禁用对应分支，因此未启用监听时不需要额外 goroutine 或轮询。
	if err := a.setState(kernelapp.Running, diagnostic.Run); err != nil {
		return a.shutdown(cancelRun, started, runnerResults, runnerCount, err, kernelapp.Failed)
	}

	// finished 记录主循环已消费的 Runner，关停时只等待剩余数量。
	finished := make(map[int]struct{}, runnerCount)
	// primary 保存触发退出的首要原因；nil 表示主循环继续监督。
	var primary error
	// 正常根取消默认以 Closed 结束，真实错误会改为 Failed 或 RestartRequired。
	finalState := kernelapp.Closed
	// Timer 只在收到首个文件事件后创建；连续保存通过安全 Stop/排空/Reset 合并为一次 Reload。
	var debounce *time.Timer
	var debounceChannel <-chan time.Time
	// 只要没有确定退出原因，就持续监督根取消、Runner、Watcher 和 debounce。
	for primary == nil {
		select {
		case <-ctx.Done():
			// 根取消先记为标准 Canceled，循环退出后统一转换为正常无错误关闭。
			primary = context.Canceled
		case result := <-runnerResults:
			// 结果通道不关闭，但只接收已知 runnerCount 次；index 标记已经消费。
			finished[result.index] = struct{}{}
			resultErr := result.err
			// 根 Context 仍有效时无错误返回意味着长期任务意外退出。
			if resultErr == nil && ctx.Err() == nil {
				resultErr = componentError(a.instances[result.index], diagnostic.Run, kernelapp.ErrRunnerExited)
			}
			// 每个 Runner 完成先产生组件事件，再根据原因决定最终状态。
			observeErr := a.emit(componentEvent(a.instances[result.index], diagnostic.Run, resultErr))
			if resultErr != nil && !(errors.Is(resultErr, context.Canceled) && ctx.Err() != nil) {
				failureObserveErr := a.emit(kernelapp.Event{Kind: kernelapp.RunnerFailed, State: kernelapp.Running, Phase: diagnostic.Run, Err: resultErr})
				primary = errors.Join(resultErr, observeErr, failureObserveErr)
				finalState = kernelapp.Failed
			} else if observeErr != nil {
				// 业务正常退出但观察链失败仍表示 Runtime 无法可靠报告状态。
				primary = observeErr
				finalState = kernelapp.Failed
			} else {
				// 根 Context 已取消时，Runner 的 nil 或 context.Canceled 都属于正常协作退出。
				primary = context.Canceled
			}
		case err, ok := <-watchErrors:
			// 关闭后的通道必须置 nil，否则 select 会持续立即命中零值。
			if !ok {
				watchErrors = nil
				continue
			}
			// Watcher 只发送非 nil 错误；防御 nil 以免无原因退出。
			if err != nil {
				primary = err
				finalState = kernelapp.Failed
			}
		case _, ok := <-watchEvents:
			// 事件内容只表示来源和时间，Reload 总是重新加载全部来源。
			if !ok {
				watchEvents = nil
				continue
			}
			// 首个事件创建 timer；后续事件安全重置同一 timer 合并保存风暴。
			if debounce == nil {
				debounce = time.NewTimer(a.options.reloadDebounce)
			} else {
				// Stop 返回 false 时 timer 可能已触发，先非阻塞排空旧 tick 再 Reset。
				if !debounce.Stop() {
					select {
					case <-debounce.C:
					default:
					}
				}
				debounce.Reset(a.options.reloadDebounce)
			}
			// 只有赋值后的 channel 才会启用下面 debounce select 分支。
			debounceChannel = debounce.C
		case <-debounceChannel:
			// 单次 tick 消费后先禁用分支，新的文件事件才能再次安排 Reload。
			debounceChannel = nil
			if ctx.Err() != nil {
				primary = context.Canceled
				continue
			}
			// 关停信号优先于生成新候选，避免 Shutdown 与 Reload 同时修改组件状态。
			err := a.reload(runCtx)
			// 根取消传播的 Canceled 属于正常关闭；Reload 自身超时则由 reloadFailure 包装。
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				primary = context.Canceled
				finalState = kernelapp.Closed
			} else if errors.Is(err, kernelapp.ErrRestartRequired) {
				// 候选有效但组件不能原地应用，交给外部监督器重启进程。
				primary = err
				finalState = kernelapp.RestartRequired
			} else if err != nil {
				// 观察失败或其他 Runtime 错误使应用进入 Failed。
				primary = err
				finalState = kernelapp.Failed
			}
		}
	}
	// 主循环结束后停止仍存在的 timer，避免其资源滞留到函数退出。
	if debounce != nil {
		debounce.Stop()
	}
	// 根 Context 协作取消不应作为应用错误返回。
	if errors.Is(primary, context.Canceled) {
		primary = nil
	}
	// 已消费结果不再等待，其余 Runner 在 shutdown 中确认退出后才允许 Close。
	return a.shutdown(cancelRun, started, runnerResults, runnerCount-len(finished), primary, finalState)
}

// reload 构建完整候选快照，仅调用受变化配置影响的组件，并在全部接受后发布快照。
func (a *Application) reload(ctx context.Context) error {
	// 一次候选加载和全部 Reloader 调用共享同一个总预算。
	reloadCtx, cancelReload := context.WithTimeout(ctx, a.options.reloadTimeout)
	defer cancelReload()
	// 每次重载都创建全新的 Loader/Koanf 实例。这样候选失败不会原地污染当前配置，
	// 也规避 Koanf Load/Get 并发访问需要额外同步的问题。
	candidate, err := a.runtime.dependencies.Loader.Load(reloadCtx, a.snapshot.Version+1, a.options.sources, a.plan.Configs)
	if err != nil {
		// 父运行 Context 取消时直接传播，关停路径不将其记录为候选拒绝。
		if errors.Is(reloadCtx.Err(), context.Canceled) {
			return reloadCtx.Err()
		}
		// Deadline 等 Context 错误是 Runtime 失败，需要发布 Reload 失败并退出。
		if contextErr := reloadCtx.Err(); contextErr != nil {
			return a.reloadFailure(fmt.Errorf("load reload candidate: %w", contextErr))
		}
		// 普通配置语法或校验错误只拒绝候选，保持当前快照和运行中组件不变。
		if observeErr := a.emit(kernelapp.Event{Kind: kernelapp.ConfigurationFail, State: a.State(), Phase: diagnostic.ConfigValidate, Err: err}); observeErr != nil {
			return observeErr
		}
		// 候选拒绝已被观察，应用继续使用旧配置运行。
		return nil
	}
	// Loader 成功后仍检查共享 Context，避免把刚超时的候选交给组件。
	if contextErr := reloadCtx.Err(); contextErr != nil {
		if errors.Is(contextErr, context.Canceled) {
			return contextErr
		}
		return a.reloadFailure(fmt.Errorf("load reload candidate: %w", contextErr))
	}
	// 使用规范化 JSON 摘要按配置类型比较，只通知真正依赖变化配置的组件。
	changed := make(map[reflect.Type]struct{})
	// Plan.Configs 是完整声明集合；缺失摘要也视为变化以保持保守安全。
	for _, declaration := range a.plan.Configs {
		oldHash, oldOK := a.snapshot.Hash(declaration.Type)
		newHash, newOK := candidate.Snapshot.Hash(declaration.Type)
		if !oldOK || !newOK || oldHash != newHash {
			changed[declaration.Type] = struct{}{}
		}
	}
	// 没有任何配置内容变化时无需调用组件或增加当前快照版本。
	if len(changed) == 0 {
		return nil
	}
	// 实例按依赖正序处理，使依赖组件先看到候选，再由消费者应用。
	for _, instance := range a.instances {
		affected := false
		// Provider 只有直接声明的配置依赖参与影响判断。
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
		// 受影响组件必须显式实现 Reloader，否则不能证明内存状态与候选一致。
		reloader, ok := instance.Value.(reloadcontract.Reloader)
		if !ok {
			// 组件无法证明可安全原地更新时，宁可请求重启，也不让新旧配置混用。
			return a.reloadFailure(kernelapp.ErrRestartRequired)
		}
		// result 在 guarded 调用内赋值，panic 或 error 都不会被解释为 Applied。
		var result reloadcontract.Result
		if err := guarded(func() error {
			var callErr error
			result, callErr = reloader.Reload(reloadCtx, candidate.Snapshot)
			return callErr
		}); err != nil {
			// 父取消优先作为正常关停信号传播。
			if errors.Is(reloadCtx.Err(), context.Canceled) {
				return reloadCtx.Err()
			}
			return a.reloadFailure(componentError(instance, diagnostic.Reload, err))
		}
		// 调用返回后检查总预算，覆盖组件忽略 Context 但最终返回的超时情况。
		if err := reloadCtx.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			return a.reloadFailure(componentError(instance, diagnostic.Reload, err))
		}
		// 只有三个项目枚举值合法，未知结果作为组件错误拒绝候选。
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
	// 快照提升后发布成功事件；观察失败会让上层退出，不能静默隐藏。
	return a.emit(kernelapp.Event{Kind: kernelapp.ConfigurationLoad, State: a.State(), Phase: diagnostic.ConfigLoad})
}

// reloadFailure 发布 Reload 阶段失败事件，并合并原始原因与观察失败。
func (a *Application) reloadFailure(cause error) error {
	// cause 保持在返回链首位，Observer/Logger panic 作为附加错误保留。
	observeErr := a.emit(kernelapp.Event{Kind: kernelapp.ConfigurationFail, State: a.State(), Phase: diagnostic.Reload, Err: cause})
	return errors.Join(cause, observeErr)
}

// forward 按依赖正序调用可选生命周期，并统一发布事件、包装错误和检查阶段预算。
func (a *Application) forward(ctx context.Context, phase diagnostic.Phase, call func(compiled.Instance) (bool, error)) error {
	// forward 保持编译计划的稳定正序，并在每次调用后检查共享阶段超时。
	for _, instance := range a.instances {
		// call 用 called 区分“未实现接口”和“实现后成功返回”。
		called, err := call(instance)
		if !called {
			continue
		}
		// 每次真实调用都发布组件事件，错误与观察失败共同决定阶段结果。
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

// failAndClose 把 Prepare、Start 等运行前失败统一导向完整 shutdown。
func (a *Application) failAndClose(parent context.Context, started []bool, cause error) error {
	// 即使启动前失败也复用 shutdown，确保所有已构造 Closer 恰好进入统一释放路径。
	if started == nil {
		// nil 表示尚无组件成功 Start，创建全 false 切片以复用 Stop 过滤逻辑。
		started = make([]bool, len(a.instances))
	}
	return a.shutdown(func() {}, started, nil, 0, cause, kernelapp.Failed)
}

// shutdown 恢复 Kernel 日志、取消 Runner、逆序 Stop/Close 并聚合全部失败。
func (a *Application) shutdown(cancel context.CancelFunc, started []bool, runnerResults <-chan runnerResult, runnersRemaining int, primary error, final kernelapp.State) error {
	// 关闭阶段开始即恢复 Kernel 基线；业务组件持有的是具体替换实例，仍可在自身 Close 前使用。
	if a.kernelLoggerReplaced {
		a.runtime.dependencies.Logger.Restore()
		a.kernelLoggerReplaced = false
	}
	// 随后取消 Runner，并使用独立后台 Context 关闭资源；父 Context 往往已经取消，直接复用
	// 会让 Stop/Close 没有执行清理的时间窗口。
	cancel()
	// 独立 Background Context 保证即使父 Context 已取消，清理仍拥有有限执行窗口。
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), a.options.shutdownTimeout)
	defer shutdownCancel()
	// 保留首要故障，同时聚合 Stop、Runner 等待和 Close 错误，避免后续清理故障被覆盖。
	errorsList := make([]error, 0)
	if primary != nil {
		errorsList = append(errorsList, primary)
	}
	shutdownFailed := false
	// Stopping 状态发布失败不会跳过真实资源清理，但会强制最终 Failed。
	if err := a.setState(kernelapp.Stopping, diagnostic.Stop); err != nil {
		errorsList = append(errorsList, err)
		shutdownFailed = true
	}
	// Stop 只面向成功启动的组件，并按依赖逆序执行。
	for index := len(a.instances) - 1; index >= 0; index-- {
		// 只 Stop 成功 Start 的实例；未启动组件仍会在 Close 阶段释放构造资源。
		if started != nil && (index >= len(started) || !started[index]) {
			continue
		}
		// 没有 Stopper 能力的组件跳过本阶段。
		stopper, ok := a.instances[index].Value.(lifecycle.Stopper)
		if !ok {
			continue
		}
		// guarded 隔离组件 panic，所有 Stop 共享 shutdown 总预算。
		callErr := guarded(func() error { return stopper.Stop(shutdownCtx) })
		if callErr == nil {
			// 组件可能忽略 Context 并在超时后返回 nil，因此成功后仍检查预算。
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
			// 每消费一个结果即递减剩余计数，并发布最终 Runner 组件事件。
			runnersRemaining--
			if err := a.emit(componentEvent(a.instances[result.index], diagnostic.Run, result.err)); err != nil {
				errorsList = append(errorsList, err)
				shutdownFailed = true
			}
			if result.err != nil && !errors.Is(result.err, context.Canceled) {
				// 关停后的 context.Canceled 是协作退出，其他错误仍属于清理失败。
				errorsList = append(errorsList, result.err)
				shutdownFailed = true
			}
		case <-shutdownCtx.Done():
			// 总预算耗尽后不再无限等待不协作的 Runner，记录超时并进入 Close。
			errorsList = append(errorsList, fmt.Errorf("wait for runners: %w", shutdownCtx.Err()))
			shutdownFailed = true
			runnersRemaining = 0
		}
	}
	// Runner 已停止或预算耗尽后才进入 Closing，避免正常后台任务访问已释放依赖。
	if err := a.setState(kernelapp.Closing, diagnostic.Close); err != nil {
		errorsList = append(errorsList, err)
		shutdownFailed = true
	}
	// Close 面向所有构造成功的实例，不受 started 标记限制。
	for index := len(a.instances) - 1; index >= 0; index-- {
		// 所有构造成功且实现 Closer 的实例都必须逆序关闭。
		closer, ok := a.instances[index].Value.(lifecycle.Closer)
		if !ok {
			continue
		}
		callErr := guarded(func() error { return closer.Close(shutdownCtx) })
		if callErr == nil {
			// 与 Stop 相同，返回 nil 后仍检查共享 shutdown Context。
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
	// 任一 Stop、等待、Close 或观察失败都覆盖原计划终态为 Failed。
	if shutdownFailed {
		final = kernelapp.Failed
	}
	// 先写原子状态再发布最终事件，使 Observer 回调读取 State 时得到同一值。
	a.stateValue.Store(uint32(final))
	if err := a.emit(kernelapp.Event{Kind: kernelapp.StateChanged, State: final}); err != nil {
		errorsList = append(errorsList, err)
		final = kernelapp.Failed
		// 最终事件发布失败无法再递归发布新事件，只修正可查询状态并返回错误。
		a.stateValue.Store(uint32(final))
	}
	return errors.Join(errorsList...)
}

// setState 原子更新 Application 状态并同步发布对应阶段事件。
func (a *Application) setState(state kernelapp.State, phase diagnostic.Phase) error {
	// Store 先于 emit，保证观察回调和并发读取者看到新状态。
	a.stateValue.Store(uint32(state))
	return a.emit(kernelapp.Event{Kind: kernelapp.StateChanged, State: state, Phase: phase})
}

// guarded 在当前 goroutine 调用用户生命周期方法，并把 panic 转为项目错误。
func guarded(call func() error) (err error) {
	// 生命周期方法属于用户代码边界，panic 必须转换为带堆栈的项目错误并进入回滚流程。
	defer func() {
		if value := recover(); value != nil {
			err = diagnostic.NewPanicError(value)
		}
	}()
	// 正常 error 原样返回，外层根据组件元数据补充阶段上下文。
	return call()
}

// componentEvent 根据实例元数据创建一次生命周期观察事件。
func componentEvent(instance compiled.Instance, phase diagnostic.Phase, err error) kernelapp.Event {
	// Provider 元数据在编译期冻结，事件不需要从运行期实例反射推断所有者。
	return kernelapp.Event{Kind: kernelapp.ComponentEvent, Phase: phase, Module: instance.Provider.Module, Component: instance.Provider.Type.String(), Err: err}
}

// componentError 为组件错误补充模块、具体类型、构造函数和阶段信息。
func componentError(instance compiled.Instance, phase diagnostic.Phase, err error) error {
	// Cause 保留原始错误，调用方仍可通过 errors.Is/As 识别取消或业务错误。
	return &diagnostic.ComponentError{Module: instance.Provider.Module, Component: instance.Provider.Type.String(), Provider: instance.Provider.Name, Phase: phase, Cause: err}
}

// hasWatchSource 判断来源集合中是否至少有一个文件等可监听来源。
func hasWatchSource(sources []config.Source) bool {
	// 只做接口能力探测，不读取描述或启动 watcher。
	for _, source := range sources {
		if _, ok := source.(config.WatchSource); ok {
			return true
		}
	}
	// 没有 WatchSource 时 Runtime 即使启用 watch 选项也不会创建 goroutine。
	return false
}
