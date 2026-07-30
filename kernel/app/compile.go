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

// compilation 是 Compile 与 Build 共用的内部结果，既保留静态 Plan，也保留构造所需的
// 强类型配置值。它不进入公共 API，避免 reflect.Value 泄漏给用户。
type compilation struct {
	plan   *compiled.Plan
	loaded loading.Loaded
	opts   options
}

// Compile 完成模块注册、依赖图校验和配置加载，但不构造任何组件。
// 它适合在启动前验证架构或导出图，不会留下需要关闭的资源。
func Compile(optionValues ...Option) (*Plan, error) {
	result, err := compileContext(context.Background(), optionValues...)
	if err != nil {
		return nil, err
	}
	return &Plan{compiled: result.plan, graph: result.plan.Graph, loaded: result.loaded.Snapshot}, nil
}

// Build 在 Compile 流水线之后事务性构造全部组件。
// 任一 Provider 或 Observer 失败时会逆序关闭已经构造的实例，绝不返回半成品 Application。
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
	// Options 必须在注册前一次性应用，使后续每个阶段看到同一份不可变输入。
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
	// 注册只收集声明；真正的类型、可见性和环检查统一交给项目 Compiler。
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
	// Application 只依赖项目 Loader 接口，Koanf 作为内部执行引擎可被替换且不会污染公共层。
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
	// Observer 属于诊断边界，也必须被 panic 隔离，否则一次日志/指标故障会跳过资源回滚。
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
	// 序号在单个 Application 内原子递增，让并发来源的事件仍可被可靠排序。
	event.Sequence = a.sequence.Add(1)
	event.Time = time.Now().UTC()
	if event.State == Created {
		event.State = a.State()
	}
	return observe(a.options.observer, event)
}

func closeConstructed(ctx context.Context, instances []compiled.Instance, cause error) error {
	// 构造顺序是依赖在前、消费者在后，释放必须反向执行，避免消费者关闭时依赖已失效。
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
