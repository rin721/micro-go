// Package runtime 提供 internal/kernel/app 契约的默认单进程实现。
package runtime

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiled"
	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/di"
	"github.com/rin721/micro-go/internal/kernel/module"
)

// Option 配置编译和运行行为。返回错误使非法超时等问题在任何资源构造前失败。
type Option func(*options) error

type options struct {
	modules         []module.Module
	sources         []config.Source
	observer        kernelapp.Observer
	startupTimeout  time.Duration
	shutdownTimeout time.Duration
	reloadTimeout   time.Duration
	watch           bool
	reloadDebounce  time.Duration
}

func defaults() options {
	return options{startupTimeout: 15 * time.Second, shutdownTimeout: 15 * time.Second, reloadTimeout: 15 * time.Second, reloadDebounce: 200 * time.Millisecond}
}

// WithModules 按声明顺序添加模块；该顺序也是稳定拓扑排序的平局规则之一。
func WithModules(modules ...module.Module) Option {
	return func(value *options) error { value.modules = append(value.modules, modules...); return nil }
}

// WithConfigSources 按覆盖优先级添加配置源，后声明的来源覆盖前者。
func WithConfigSources(sources ...config.Source) Option {
	return func(value *options) error { value.sources = append(value.sources, sources...); return nil }
}

// WithObserver 设置同步 Observer；未设置时 Runtime 不会创建事件队列或 goroutine。
func WithObserver(observer kernelapp.Observer) Option {
	return func(value *options) error { value.observer = observer; return nil }
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

func durationOption(name string, timeout time.Duration, apply func(*options)) Option {
	return func(value *options) error {
		if timeout <= 0 {
			return errors.New(name + " timeout must be positive")
		}
		apply(value)
		return nil
	}
}

// Plan 是编译完成后的只读结果。内部计划不公开，防止调用方绕过 Application
// 直接操作构造引擎；对外只提供项目自有依赖图副本。
type Plan struct {
	compiled *compiled.Plan
	graph    di.Graph
	loaded   config.Snapshot
}

// DependencyGraph 返回节点和边切片的副本，调用方修改结果不会破坏已编译计划。
func (p *Plan) DependencyGraph() di.Graph {
	if p == nil {
		return di.Graph{}
	}
	return di.Graph{Nodes: append([]di.Node(nil), p.graph.Nodes...), Edges: append([]di.Edge(nil), p.graph.Edges...)}
}

// Application 拥有已构造实例、当前配置快照以及完整生命周期状态。
// 它只能 Run 一次，因为组件实例和资源关闭语义都是一次性的。
type Application struct {
	stateValue atomic.Uint32
	sequence   atomic.Uint64
	plan       *compiled.Plan
	instances  []compiled.Instance
	snapshot   config.Snapshot
	options    options
	runtime    *Runtime
	runStarted atomic.Bool
}

// State 无锁返回当前状态，适合 Observer、健康检查和并发诊断读取。
func (a *Application) State() kernelapp.State { return kernelapp.State(a.stateValue.Load()) }

// 编译期断言把默认实现与 Kernel 契约绑定，避免目录迁移后方法签名悄然漂移。
var _ kernelapp.Plan = (*Plan)(nil)
var _ kernelapp.Application = (*Application)(nil)
