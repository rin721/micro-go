package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rin721/micro-go/internal/config/koanfadapter"
	"github.com/rin721/micro-go/internal/config/loading"
	"github.com/rin721/micro-go/internal/di/compiled"
	"github.com/rin721/micro-go/internal/di/compiler"
	"github.com/rin721/micro-go/internal/di/constructor"
	"github.com/rin721/micro-go/internal/di/digadapter"
	"github.com/rin721/micro-go/internal/registration"
	"github.com/rin721/micro-go/kernel/diagnostic"
)

type compilation struct {
	plan   *compiled.Plan
	loaded loading.Loaded
	opts   options
}

func Compile(optionValues ...Option) (*Plan, error) {
	result, err := compileContext(context.Background(), optionValues...)
	if err != nil {
		return nil, err
	}
	return &Plan{compiled: result.plan, graph: result.plan.Graph, loaded: result.loaded.Snapshot}, nil
}

func Build(ctx context.Context, optionValues ...Option) (*Application, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := compileContext(ctx, optionValues...)
	if err != nil {
		return nil, err
	}
	if err := observe(result.opts.observer, Event{Time: time.Now().UTC(), Kind: StateChanged, State: Constructing, Phase: diagnostic.Construct}); err != nil {
		return nil, err
	}
	var engine constructor.Engine = digadapter.New()
	instances, err := engine.Construct(ctx, result.plan, result.loaded.Values)
	if err != nil {
		return nil, err
	}
	application := &Application{plan: result.plan, instances: instances, snapshot: result.loaded.Snapshot, options: result.opts}
	application.stateValue.Store(uint32(Built))
	if err := application.emit(Event{Kind: StateChanged, State: Built}); err != nil {
		return nil, closeConstructed(ctx, instances, err)
	}
	return application, nil
}

func compileContext(ctx context.Context, optionValues ...Option) (compilation, error) {
	opts := defaults()
	for _, option := range optionValues {
		if option == nil {
			return compilation{}, fmt.Errorf("application option is nil")
		}
		if err := option(&opts); err != nil {
			return compilation{}, fmt.Errorf("apply application option: %w", err)
		}
	}
	if err := observe(opts.observer, Event{Time: time.Now().UTC(), Kind: StateChanged, State: Registering, Phase: diagnostic.ModuleRegister}); err != nil {
		return compilation{}, err
	}
	collection, err := registration.Collect(opts.modules)
	if err != nil {
		return compilation{}, &diagnostic.ComponentError{Phase: diagnostic.ModuleRegister, Cause: err}
	}
	if err := observe(opts.observer, Event{Time: time.Now().UTC(), Kind: StateChanged, State: Compiling, Phase: diagnostic.GraphCompile}); err != nil {
		return compilation{}, err
	}
	plan, err := compiler.Compile(collection)
	if err != nil {
		return compilation{}, &diagnostic.ComponentError{Phase: diagnostic.GraphCompile, Cause: err}
	}
	var loader loading.Loader = koanfadapter.New()
	loaded, err := loader.Load(ctx, 1, opts.sources, plan.Configs)
	if err != nil {
		return compilation{}, &diagnostic.ComponentError{Phase: diagnostic.ConfigLoad, Cause: err}
	}
	if err := observe(opts.observer, Event{Time: time.Now().UTC(), Kind: ConfigurationLoad, State: Compiling, Phase: diagnostic.ConfigLoad}); err != nil {
		return compilation{}, err
	}
	if err := observe(opts.observer, Event{Time: time.Now().UTC(), Kind: GraphCompiled, State: Compiling, Phase: diagnostic.GraphCompile}); err != nil {
		return compilation{}, err
	}
	return compilation{plan: plan, loaded: loaded, opts: opts}, nil
}

func observe(observer Observer, event Event) (err error) {
	if observer == nil {
		return nil
	}
	defer func() {
		if value := recover(); value != nil {
			err = &diagnostic.ComponentError{Phase: diagnostic.Observe, Cause: diagnostic.NewPanicError(value)}
		}
	}()
	observer.Observe(event)
	return nil
}

func (a *Application) emit(event Event) error {
	event.Sequence = a.sequence.Add(1)
	event.Time = time.Now().UTC()
	if event.State == Created {
		event.State = a.State()
	}
	return observe(a.options.observer, event)
}

func closeConstructed(ctx context.Context, instances []compiled.Instance, cause error) error {
	errorsList := []error{cause}
	for index := len(instances) - 1; index >= 0; index-- {
		closer, ok := instances[index].Value.(interface{ Close(context.Context) error })
		if !ok {
			continue
		}
		if err := func() (err error) {
			defer func() {
				if value := recover(); value != nil {
					err = diagnostic.NewPanicError(value)
				}
			}()
			return closer.Close(ctx)
		}(); err != nil {
			errorsList = append(errorsList, err)
		}
	}
	return errors.Join(errorsList...)
}
