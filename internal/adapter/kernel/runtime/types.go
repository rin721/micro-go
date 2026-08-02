// Package runtime 提供 internal/kernel/app 契约的默认单进程实现。
package runtime

import (
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiled"
	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/di"
	"github.com/rin721/micro-go/internal/kernel/module"
	capabilitylogging "github.com/rin721/micro-go/types/capability/logging"
)

// Option 配置编译和运行行为。返回错误使非法超时等问题在任何资源构造前失败。
type Option func(*options) error

// options 是所有 Option 在注册前汇总出的单次 Build 不可变输入。
type options struct {
	// modules 按组合根声明顺序保存静态模块。
	modules []module.Module
	// sources 按低到高覆盖优先级保存配置来源。
	sources []config.Source
	// observer 是可选同步观察者，nil 表示仅发布 Kernel 日志。
	observer kernelapp.Observer
	// startupTimeout 限制 Prepare 与 Start 的总时长。
	startupTimeout time.Duration
	// shutdownTimeout 限制 Stop、等待 Runner 与 Close 的总时长。
	shutdownTimeout time.Duration
	// reloadTimeout 限制一次完整候选加载和应用。
	reloadTimeout time.Duration
	// watch 决定是否为可监听 Source 启动文件事件循环。
	watch bool
	// reloadDebounce 合并连续文件事件。
	reloadDebounce time.Duration
	// kernelLoggerReplacement 保存显式接管运行期日志的具体实现类型。
	kernelLoggerReplacement reflect.Type
}

// defaults 集中返回所有 Runtime 时间边界的默认值。
func defaults() options {
	// 默认值只在此处出现，Option 随后按调用顺序覆盖对应字段。
	return options{startupTimeout: 15 * time.Second, shutdownTimeout: 15 * time.Second, reloadTimeout: 15 * time.Second, reloadDebounce: 200 * time.Millisecond}
}

// WithModules 按声明顺序添加模块；该顺序也是稳定拓扑排序的平局规则之一。
func WithModules(modules ...module.Module) Option {
	// append 保留既有模块和本次参数的准确顺序；合法性由 Collector 统一校验。
	return func(value *options) error { value.modules = append(value.modules, modules...); return nil }
}

// WithConfigSources 按覆盖优先级添加配置源，后声明的来源覆盖前者。
func WithConfigSources(sources ...config.Source) Option {
	// 来源不在 Option 阶段读取，Build 的 Loader 才创建一致候选。
	return func(value *options) error { value.sources = append(value.sources, sources...); return nil }
}

// WithObserver 设置同步 Observer；未设置时 Runtime 不会创建事件队列或 goroutine。
func WithObserver(observer kernelapp.Observer) Option {
	// Observer 生命周期由调用方拥有，Runtime 只保存并同步调用引用。
	return func(value *options) error { value.observer = observer; return nil }
}

// WithKernelLoggerReplacement 显式指定 DI 图中接管 Kernel 运行期日志的具体 Logger 类型。
// 该类型必须由 Provider 构造并绑定为 logging.Logger；未设置时 Kernel 始终使用自己的基线。
func WithKernelLoggerReplacement[Implementation capabilitylogging.Logger]() Option {
	// 泛型参数只提取具体实现类型，不创建 Logger 实例。
	implementation := reflect.TypeOf((*Implementation)(nil)).Elem()
	return func(value *options) error {
		// 接口类型无法对应唯一 Provider 实例，必须由调用方指定具体实现。
		if implementation.Kind() == reflect.Interface {
			return fmt.Errorf("kernel logger replacement must be a concrete type, got %s", implementation)
		}
		// 一个 Build 只能选择一个替换目标，禁止后续 Option 静默覆盖先前决策。
		if value.kernelLoggerReplacement != nil {
			return fmt.Errorf("kernel logger replacement already set to %s", value.kernelLoggerReplacement)
		}
		// 类型仅作为编译期校验和构造后查找键，不泄漏给业务组件。
		value.kernelLoggerReplacement = implementation
		return nil
	}
}

// WithConfigWatch 启用可监听 Source 的文件变化；没有 WatchSource 时不会启动 goroutine。
func WithConfigWatch() Option { return func(value *options) error { value.watch = true; return nil } }

// WithStartupTimeout 设置 Prepare 和 Start 共享的总超时。
func WithStartupTimeout(timeout time.Duration) Option {
	return durationOption("startup", timeout, func(value *options) { value.startupTimeout = timeout })
}

// WithShutdownTimeout 设置 Stop、等待 Runner 和 Close 共享的总超时。
func WithShutdownTimeout(timeout time.Duration) Option {
	return durationOption("shutdown", timeout, func(value *options) { value.shutdownTimeout = timeout })
}

// WithReloadTimeout 设置一次候选加载和全部 Reloader 调用共享的总超时。
// 超时依赖 Source 与组件遵守 Context；Runtime 不遗留无法回收的强制中断 goroutine。
func WithReloadTimeout(timeout time.Duration) Option {
	return durationOption("reload", timeout, func(value *options) { value.reloadTimeout = timeout })
}

// WithReloadDebounce 设置文件事件去抖时间，避免编辑器一次保存触发多次全量候选构建。
func WithReloadDebounce(timeout time.Duration) Option {
	return durationOption("reload debounce", timeout, func(value *options) { value.reloadDebounce = timeout })
}

// durationOption 统一校验所有必须为正数的时间 Option，并在成功后应用字段更新。
func durationOption(name string, timeout time.Duration, apply func(*options)) Option {
	return func(value *options) error {
		// 零值或负值会造成立即超时或无界语义混乱，因此不允许作为禁用方式。
		if timeout <= 0 {
			return errors.New(name + " timeout must be positive")
		}
		// 只有校验通过才调用具体字段 setter，失败不会留下部分 Option 状态。
		apply(value)
		return nil
	}
}

// Plan 是编译完成后的只读结果。内部计划不公开，防止调用方绕过 Application
// 直接操作构造引擎；对外只提供项目自有依赖图副本。
type Plan struct {
	// compiled 保留 Build 所需的内部反射计划，不对调用方公开。
	compiled *compiled.Plan
	// graph 是从内部计划投影出的项目自有只读图。
	graph di.Graph
	// loaded 是 Compile 时成功加载的配置快照。
	loaded config.Snapshot
}

// DependencyGraph 返回节点和边切片的副本，调用方修改结果不会破坏已编译计划。
func (p *Plan) DependencyGraph() di.Graph {
	// nil Plan 返回合法空图，避免诊断调用因 nil receiver panic。
	if p == nil {
		return di.Graph{}
	}
	// 节点和边分别复制底层切片，结构体元素本身按值复制。
	return di.Graph{Nodes: append([]di.Node(nil), p.graph.Nodes...), Edges: append([]di.Edge(nil), p.graph.Edges...)}
}

// Application 拥有已构造实例、当前配置快照以及完整生命周期状态。
// 它只能 Run 一次，因为组件实例和资源关闭语义都是一次性的。
type Application struct {
	// stateValue 允许观察方无锁读取当前单向状态。
	stateValue atomic.Uint32
	// sequence 为所有事件分配当前 Application 内唯一递增序号。
	sequence atomic.Uint64
	// plan 保存生命周期顺序和 Provider 元数据。
	plan *compiled.Plan
	// instances 按依赖正序保存全部成功构造的组件。
	instances []compiled.Instance
	// snapshot 是当前已发布配置事实，只有完整 Reload 成功后替换。
	snapshot config.Snapshot
	// options 保存本次 Build 已冻结的模块、来源、观察者和时间边界。
	options options
	// runtime 提供日志、Watcher、Loader 等显式依赖。
	runtime *Runtime
	// kernelLoggerReplaced 标记关闭前是否必须 Restore 基线 Logger。
	kernelLoggerReplaced bool
	// runStarted 以原子 CAS 保证 Application.Run 最多成功进入一次。
	runStarted atomic.Bool
}

// State 无锁返回当前状态，适合 Observer、健康检查和并发诊断读取。
func (a *Application) State() kernelapp.State { return kernelapp.State(a.stateValue.Load()) }

// 编译期断言把默认 Plan 实现与 Kernel 契约绑定。
var _ kernelapp.Plan = (*Plan)(nil)

// 编译期断言把默认 Application 实现与 Kernel 契约绑定。
var _ kernelapp.Application = (*Application)(nil)
