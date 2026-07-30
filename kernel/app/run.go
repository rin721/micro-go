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

func (a *Application) Run(ctx context.Context) error {
	if a == nil {
		return errors.New("application is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !a.runStarted.CompareAndSwap(false, true) {
		return ErrAlreadyRun
	}
	if a.State() != Built {
		return fmt.Errorf("cannot run application in state %s", a.State())
	}

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
		go func(index int, instance compiled.Instance, runner lifecycle.Runner) {
			err := guarded(func() error { return runner.Run(runCtx) })
			if err != nil {
				err = componentError(instance, diagnostic.Run, err)
			}
			runnerResults <- runnerResult{index: index, err: err}
		}(index, instance, runner)
	}

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
	var loader loading.Loader = koanfadapter.New()
	candidate, err := loader.Load(ctx, a.snapshot.Version+1, a.options.sources, a.plan.Configs)
	if err != nil {
		if observeErr := a.emit(Event{Kind: ConfigurationFail, State: Running, Phase: diagnostic.ConfigValidate, Err: err}); observeErr != nil {
			return observeErr
		}
		return nil
	}
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
	a.snapshot = candidate.Snapshot
	return a.emit(Event{Kind: ConfigurationLoad, State: Running, Phase: diagnostic.ConfigLoad})
}

func (a *Application) forward(ctx context.Context, phase diagnostic.Phase, call func(compiled.Instance) (bool, error)) error {
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
	if started == nil {
		started = make([]bool, len(a.instances))
	}
	return a.shutdown(func() {}, started, nil, 0, cause, Failed)
}

func (a *Application) shutdown(cancel context.CancelFunc, started []bool, runnerResults <-chan runnerResult, runnersRemaining int, primary error, final State) error {
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), a.options.shutdownTimeout)
	defer shutdownCancel()
	errorsList := make([]error, 0)
	if primary != nil {
		errorsList = append(errorsList, primary)
	}
	_ = a.setState(Stopping, diagnostic.Stop)
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
