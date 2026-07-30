// Package app 负责协调配置、依赖构造、生命周期、运行监督和退出。
package app

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/rin721/micro-go/internal/di/compiled"
	"github.com/rin721/micro-go/kernel/config"
	"github.com/rin721/micro-go/kernel/di"
	"github.com/rin721/micro-go/kernel/diagnostic"
	"github.com/rin721/micro-go/kernel/module"
)

// State 是 Application 的单向运行状态。
// 状态通过 atomic 保存，Observer 和外部健康检查无需持有应用内部锁即可读取。
type State uint8

const (
	// Created 表示尚未进入注册流程的概念初始态。
	Created State = iota
	// Registering 表示正在收集模块声明。
	Registering
	// Compiling 表示正在校验配置和依赖图。
	Compiling
	// Constructing 表示正在事务性构造组件。
	Constructing
	// Built 表示全部组件已构造，但生命周期尚未启动。
	Built
	// Preparing 表示正在按依赖正序准备组件。
	Preparing
	// Starting 表示正在按依赖正序启动组件。
	Starting
	// Running 表示 Runner 已受监督且配置监听已就绪。
	Running
	// Stopping 表示正在逆序停止已启动组件。
	Stopping
	// Closing 表示正在逆序释放所有已构造组件。
	Closing
	// Closed 表示应用已正常关闭。
	Closed
	// Failed 表示应用因构造、生命周期、Runner、Reload 或关闭错误退出。
	Failed
	// RestartRequired 表示配置变化无法原地应用，应用已选择优雅退出。
	RestartRequired
)

// String 返回稳定的状态名称，未知值不会导致数组越界。
func (s State) String() string {
	names := [...]string{"Created", "Registering", "Compiling", "Constructing", "Built", "Preparing", "Starting", "Running", "Stopping", "Closing", "Closed", "Failed", "RestartRequired"}
	if int(s) >= len(names) {
		return "Unknown"
	}
	return names[s]
}

var (
	// ErrAlreadyRun 表示同一个 Application 被重复运行。Application 是一次性状态机，
	// 重复运行会使资源所有权和生命周期次数变得不确定，因此明确拒绝。
	ErrAlreadyRun = errors.New("application can only run once")
	// ErrRestartRequired 表示候选配置有效，但至少一个受影响组件要求通过重启应用。
	ErrRestartRequired = errors.New("configuration change requires application restart")
)

// EventKind 对 Observer 可见的事件进行稳定分类。
type EventKind string

const (
	// StateChanged 表示 Application 状态发生变化。
	StateChanged EventKind = "state.changed"
	// ConfigurationLoad 表示初始配置或候选配置已经成功加载。
	ConfigurationLoad EventKind = "config.loaded"
	// ConfigurationFail 表示候选配置无效，当前快照仍保持不变。
	ConfigurationFail EventKind = "config.failed"
	// GraphCompiled 表示依赖图编译完成。
	GraphCompiled EventKind = "graph.compiled"
	// ComponentEvent 表示某个组件进入生命周期阶段。
	ComponentEvent EventKind = "component.lifecycle"
	// RunnerFailed 表示受监督 Runner 返回错误。
	RunnerFailed EventKind = "runner.failed"
)

// Event 是同步 Observer 收到的项目自有只读事件。
// Err 已经过框架归一化，不包含第三方容器对象；Sequence 只在 Application 建成后递增。
type Event struct {
	// Sequence 是单个 Application 内的递增序号。
	Sequence uint64
	// Time 是事件生成的 UTC 时间。
	Time time.Time
	// Kind 是事件稳定分类。
	Kind EventKind
	// State 是事件发生时或转换后的应用状态。
	State State
	// Phase 是对应的框架执行阶段。
	Phase diagnostic.Phase
	// Module 在组件事件中标识所属模块。
	Module string
	// Component 在组件事件中标识具体类型。
	Component string
	// Err 是已经过框架边界归一化的错误。
	Err error
}

// Observer 同步观察框架事件。实现必须快速返回且不能修改 Application；框架会捕获 panic，
// 避免诊断代码绕过正常回滚流程。
type Observer interface{ Observe(Event) }

// Option 配置编译和运行行为。返回错误使非法超时等问题在任何资源构造前失败。
type Option func(*options) error

type options struct {
	modules         []module.Module
	sources         []config.Source
	observer        Observer
	startupTimeout  time.Duration
	shutdownTimeout time.Duration
	watch           bool
	reloadDebounce  time.Duration
}

func defaults() options {
	return options{startupTimeout: 15 * time.Second, shutdownTimeout: 15 * time.Second, reloadDebounce: 200 * time.Millisecond}
}

// WithModules 按声明顺序添加模块；该顺序也是稳定拓扑排序的平局规则之一。
func WithModules(modules ...module.Module) Option {
	return func(value *options) error { value.modules = append(value.modules, modules...); return nil }
}

// WithConfigSources 按覆盖优先级添加配置源，后声明的来源覆盖前者。
func WithConfigSources(sources ...config.Source) Option {
	return func(value *options) error { value.sources = append(value.sources, sources...); return nil }
}

// WithObserver 设置同步 Observer；未设置时框架不会创建事件队列或 goroutine。
func WithObserver(observer Observer) Option {
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
	runStarted atomic.Bool
}

// State 无锁返回当前状态，适合 Observer、健康检查和并发诊断读取。
func (a *Application) State() State { return State(a.stateValue.Load()) }
