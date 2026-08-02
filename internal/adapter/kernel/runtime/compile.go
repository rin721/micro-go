// 本文件实现 Runtime 的注册、图编译、配置加载、事务构造及构造失败回滚流水线。
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
	capabilitylogging "github.com/rin721/micro-go/types/capability/logging"
)

// compilation 是 Compile 与 Build 共用的内部结果，既保留静态 Plan，也保留构造所需的
// 强类型配置值。它不进入 Kernel 契约，避免 reflect.Value 泄漏到组合根。
type compilation struct {
	// plan 是完成全部静态校验后的冻结依赖计划。
	plan *compiled.Plan
	// loaded 是与计划配置声明对应的快照和构造注入值。
	loaded config.Loaded
	// opts 是在注册前完成应用并冻结的运行参数。
	opts options
}

// Compile 完成模块注册、依赖图校验和配置加载，但不构造任何组件。
// 它适合在启动前验证架构或导出图，不会留下需要关闭的资源。
func (r *Runtime) Compile(optionValues ...Option) (*Plan, error) {
	// 纯编译入口没有调用方 Context，使用不会自行取消的 Background。
	result, err := r.compileContext(context.Background(), optionValues...)
	if err != nil {
		return nil, err
	}
	// 对外 Plan 只保留内部计划引用和项目图/快照，无法解析实例。
	return &Plan{compiled: result.plan, graph: result.plan.Graph, loaded: result.loaded.Snapshot}, nil
}

// Build 在 Compile 流水线之后事务性构造全部组件。
// 任一 Provider、Kernel 日志或 Observer 失败时会逆序关闭已经构造的实例，绝不返回半成品 Application。
func (r *Runtime) Build(ctx context.Context, optionValues ...Option) (*Application, error) {
	// nil Context 没有取消来源，按标准库惯例转换为 Background 而不是 panic。
	if ctx == nil {
		ctx = context.Background()
	}
	// 编译阶段在任何组件构造前完成声明、图和配置的全部失败检查。
	result, err := r.compileContext(ctx, optionValues...)
	if err != nil {
		return nil, err
	}
	// Constructing 事件成功发布后才允许调用可能创建资源的 Constructor。
	if err := r.publish(result.opts.observer, kernelapp.Event{Time: time.Now().UTC(), Kind: kernelapp.StateChanged, State: kernelapp.Constructing, Phase: diagnostic.Construct}); err != nil {
		return nil, err
	}
	// Constructor 即使失败也返回已完成实例，Runtime 必须负责统一逆序回滚。
	instances, err := r.dependencies.Constructor.Construct(ctx, result.plan, result.loaded.Values)
	if err != nil {
		cause := rollbackConstructed(instances, err, result.opts.shutdownTimeout)
		publishErr := r.publish(result.opts.observer, kernelapp.Event{Time: time.Now().UTC(), Kind: kernelapp.StateChanged, State: kernelapp.Failed, Phase: diagnostic.Construct, Err: cause})
		return nil, errors.Join(cause, publishErr)
	}
	// 只有全部 Provider 成功后才创建 Application，避免向调用方泄漏半成品。
	application := &Application{plan: result.plan, instances: instances, snapshot: result.loaded.Snapshot, options: result.opts, runtime: r}
	// 显式替换目标存在时，从已构造实例中查找同一个绑定对象并切换 Kernel Manager。
	if result.opts.kernelLoggerReplacement != nil {
		replacement, findErr := findKernelLoggerReplacement(instances, result.opts.kernelLoggerReplacement)
		if findErr != nil {
			// 查找失败代表计划与实例结果不一致，关闭全部已构造资源并发布失败。
			cause := rollbackConstructed(instances, findErr, result.opts.shutdownTimeout)
			publishErr := r.publish(result.opts.observer, kernelapp.Event{Time: time.Now().UTC(), Kind: kernelapp.StateChanged, State: kernelapp.Failed, Phase: diagnostic.Construct, Err: cause})
			return nil, errors.Join(cause, publishErr)
		}
		// Replace 不接管资源所有权；Application 只记录退出时需要 Restore。
		if replaceErr := r.dependencies.Logger.Replace(replacement); replaceErr != nil {
			cause := rollbackConstructed(instances, fmt.Errorf("replace kernel logger: %w", replaceErr), result.opts.shutdownTimeout)
			publishErr := r.publish(result.opts.observer, kernelapp.Event{Time: time.Now().UTC(), Kind: kernelapp.StateChanged, State: kernelapp.Failed, Phase: diagnostic.Construct, Err: cause})
			return nil, errors.Join(cause, publishErr)
		}
		application.kernelLoggerReplaced = true
	}
	// Built 状态先原子可见，再通过 emit 分配序号并发布状态事件。
	application.stateValue.Store(uint32(kernelapp.Built))
	if err := application.emit(kernelapp.Event{Kind: kernelapp.StateChanged, State: kernelapp.Built}); err != nil {
		if application.kernelLoggerReplaced {
			// Built 事件失败时恢复基线，随后回滚仍可能依赖增强 Logger 的实例。
			r.dependencies.Logger.Restore()
			application.kernelLoggerReplaced = false
		}
		return nil, rollbackConstructed(instances, err, result.opts.shutdownTimeout)
	}
	return application, nil
}

// compileContext 应用 Option，并顺序完成模块收集、静态编译和初始配置加载。
func (r *Runtime) compileContext(ctx context.Context, optionValues ...Option) (compilation, error) {
	// Options 必须在注册前一次性应用，使后续每个阶段看到同一份不可变输入。
	opts := defaults()
	// Option 顺序具有覆盖含义；任一 nil 或失败都会在模块注册前终止。
	for _, option := range optionValues {
		if option == nil {
			return compilation{}, fmt.Errorf("application option is nil")
		}
		if err := option(&opts); err != nil {
			return compilation{}, fmt.Errorf("apply application option: %w", err)
		}
	}
	// Registering 事件本身也属于失败边界，诊断不可用时不继续隐藏后续状态。
	if err := r.publish(opts.observer, kernelapp.Event{Time: time.Now().UTC(), Kind: kernelapp.StateChanged, State: kernelapp.Registering, Phase: diagnostic.ModuleRegister}); err != nil {
		return compilation{}, err
	}
	// 注册只收集声明；真正的类型、可见性和环检查统一交给项目 Compiler。
	collection, err := r.dependencies.Collector.Collect(opts.modules)
	if err != nil {
		cause := &diagnostic.ComponentError{Phase: diagnostic.ModuleRegister, Cause: err}
		return compilation{}, errors.Join(cause, r.publish(opts.observer, kernelapp.Event{Time: time.Now().UTC(), Kind: kernelapp.StateChanged, State: kernelapp.Failed, Phase: diagnostic.ModuleRegister, Err: cause}))
	}
	// 声明冻结后进入静态编译阶段，事件顺序与实际动作保持一致。
	if err := r.publish(opts.observer, kernelapp.Event{Time: time.Now().UTC(), Kind: kernelapp.StateChanged, State: kernelapp.Compiling, Phase: diagnostic.GraphCompile}); err != nil {
		return compilation{}, err
	}
	plan, err := r.dependencies.Compiler.Compile(collection)
	if err != nil {
		cause := &diagnostic.ComponentError{Phase: diagnostic.GraphCompile, Cause: err}
		return compilation{}, errors.Join(cause, r.publish(opts.observer, kernelapp.Event{Time: time.Now().UTC(), Kind: kernelapp.StateChanged, State: kernelapp.Failed, Phase: diagnostic.GraphCompile, Err: cause}))
	}
	// 替换类型必须在冻结 Plan 中同时存在 Provider 和 logging.Logger Binding。
	if err := validateKernelLoggerReplacement(plan, opts.kernelLoggerReplacement); err != nil {
		cause := &diagnostic.ComponentError{Phase: diagnostic.GraphCompile, Cause: err}
		return compilation{}, errors.Join(cause, r.publish(opts.observer, kernelapp.Event{Time: time.Now().UTC(), Kind: kernelapp.StateChanged, State: kernelapp.Failed, Phase: diagnostic.GraphCompile, Err: cause}))
	}
	// Loader 由 Bootstrap 注入；Runtime 只依赖项目 Port，不知道候选树由 Koanf 还是其他引擎实现。
	loaded, err := r.dependencies.Loader.Load(ctx, 1, opts.sources, plan.Configs)
	if err != nil {
		cause := &diagnostic.ComponentError{Phase: diagnostic.ConfigLoad, Cause: err}
		return compilation{}, errors.Join(cause, r.publish(opts.observer, kernelapp.Event{Time: time.Now().UTC(), Kind: kernelapp.ConfigurationFail, State: kernelapp.Failed, Phase: diagnostic.ConfigLoad, Err: cause}))
	}
	// 配置和图事件只在对应事实成功后发布，Observer 不会看到未成立的状态。
	if err := r.publish(opts.observer, kernelapp.Event{Time: time.Now().UTC(), Kind: kernelapp.ConfigurationLoad, State: kernelapp.Compiling, Phase: diagnostic.ConfigLoad}); err != nil {
		return compilation{}, err
	}
	if err := r.publish(opts.observer, kernelapp.Event{Time: time.Now().UTC(), Kind: kernelapp.GraphCompiled, State: kernelapp.Compiling, Phase: diagnostic.GraphCompile}); err != nil {
		return compilation{}, err
	}
	// 三项结果作为一个值返回，Build 和 Compile 不会混用不同轮次的计划与配置。
	return compilation{plan: plan, loaded: loaded, opts: opts}, nil
}

// emit 为 Application 事件补齐单调序号、UTC 时间和必要的当前状态后交给 Runtime 发布。
func (a *Application) emit(event kernelapp.Event) error {
	// 序号在单个 Application 内原子递增，让并发来源的事件仍可被可靠排序。
	event.Sequence = a.sequence.Add(1)
	event.Time = time.Now().UTC()
	// Created 在待发布事件中表示“未显式指定”，替换为原子读取的当前状态。
	if event.State == kernelapp.Created {
		event.State = a.State()
	}
	return a.runtime.publish(a.options.observer, event)
}

// loggingContractType 是静态校验 Kernel Logger 替换 Binding 使用的项目接口类型键。
var loggingContractType = reflect.TypeOf((*capabilitylogging.Logger)(nil)).Elem()

// validateKernelLoggerReplacement 确认替换具体类型由 Provider 构造且绑定到日志契约。
func validateKernelLoggerReplacement(plan *compiled.Plan, implementation reflect.Type) error {
	// nil 表示调用方没有请求替换，Kernel 基线保持全程有效。
	if implementation == nil {
		return nil
	}
	// 先确认具体类型存在于 Provider 集合，避免只声明悬空 Binding。
	providerFound := false
	for _, provider := range plan.Providers {
		if provider.Type == implementation {
			providerFound = true
			break
		}
	}
	if !providerFound {
		return fmt.Errorf("kernel logger replacement %s has no provider", implementation)
	}
	// Binding 必须同时匹配准确日志接口和所选具体实现。
	for _, binding := range plan.Bindings {
		if binding.Contract == loggingContractType && binding.Implementation == implementation {
			return nil
		}
	}
	return fmt.Errorf("kernel logger replacement %s is not bound to %s", implementation, loggingContractType)
}

// findKernelLoggerReplacement 从构造结果中取得计划指定的同一个业务 Logger 实例。
func findKernelLoggerReplacement(instances []compiled.Instance, implementation reflect.Type) (capabilitylogging.Logger, error) {
	// Instance 与 Provider 元数据配对，因此按准确结果类型查找不会依赖容器解析。
	for _, instance := range instances {
		if instance.Provider.Type != implementation {
			continue
		}
		// 类型断言和 typed nil 检查共同防止 Manager 委托到无效实例。
		logger, ok := instance.Value.(capabilitylogging.Logger)
		if !ok || logger == nil || isNilReplacement(logger) {
			return nil, fmt.Errorf("kernel logger replacement %s produced an invalid instance", implementation)
		}
		return logger, nil
	}
	// 编译期存在但构造结果缺失代表内部不变量破坏，必须作为构造失败处理。
	return nil, fmt.Errorf("kernel logger replacement %s was not constructed", implementation)
}

// isNilReplacement 识别 capability 接口中包裹的 nil 引用值。
func isNilReplacement(logger capabilitylogging.Logger) bool {
	// 只有可为 nil 的反射 Kind 才允许调用 IsNil。
	value := reflect.ValueOf(logger)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// rollbackConstructed 使用独立有界 Context 关闭构造成功前缀并合并原始失败。
func rollbackConstructed(instances []compiled.Instance, cause error, timeout time.Duration) error {
	// 构造 Context 可能已经取消，因此清理必须拥有新的超时边界。
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return closeConstructed(ctx, instances, cause)
}

// closeConstructed 按实例逆序调用可选 Close，并保留主错误及全部清理错误。
func closeConstructed(ctx context.Context, instances []compiled.Instance, cause error) error {
	// 构造顺序是依赖在前、消费者在后，释放必须反向执行，避免消费者关闭时依赖已失效。
	errorsList := []error{cause}
	// 最后构造的消费者最先释放，确保其依赖在 Close 期间仍然可用。
	for index := len(instances) - 1; index >= 0; index-- {
		// 未实现 Close 的无资源组件无需进入清理调用。
		closer, ok := instances[index].Value.(interface{ Close(context.Context) error })
		if !ok {
			continue
		}
		// 每个组件拥有独立 panic 边界，一个异常不会阻止其余实例继续释放。
		if err := func() (err error) {
			defer func() {
				if value := recover(); value != nil {
					err = diagnostic.NewPanicError(value)
				}
			}()
			return closer.Close(ctx)
		}(); err != nil {
			// 补充准确组件和 Close 阶段上下文后追加，不覆盖先前错误。
			errorsList = append(errorsList, componentError(instances[index], diagnostic.Close, err))
		}
	}
	// Join 忽略 nil cause；有多个关闭失败时调用方仍可逐项 errors.Is/As。
	return errors.Join(errorsList...)
}
