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

type State uint8

const (
	Created State = iota
	Registering
	Compiling
	Constructing
	Built
	Preparing
	Starting
	Running
	Stopping
	Closing
	Closed
	Failed
	RestartRequired
)

func (s State) String() string {
	names := [...]string{"Created", "Registering", "Compiling", "Constructing", "Built", "Preparing", "Starting", "Running", "Stopping", "Closing", "Closed", "Failed", "RestartRequired"}
	if int(s) >= len(names) {
		return "Unknown"
	}
	return names[s]
}

var (
	ErrAlreadyRun      = errors.New("application can only run once")
	ErrRestartRequired = errors.New("configuration change requires application restart")
)

type EventKind string

const (
	StateChanged      EventKind = "state.changed"
	ConfigurationLoad EventKind = "config.loaded"
	ConfigurationFail EventKind = "config.failed"
	GraphCompiled     EventKind = "graph.compiled"
	ComponentEvent    EventKind = "component.lifecycle"
	RunnerFailed      EventKind = "runner.failed"
)

type Event struct {
	Sequence  uint64
	Time      time.Time
	Kind      EventKind
	State     State
	Phase     diagnostic.Phase
	Module    string
	Component string
	Err       error
}

type Observer interface{ Observe(Event) }

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

func WithModules(modules ...module.Module) Option {
	return func(value *options) error { value.modules = append(value.modules, modules...); return nil }
}
func WithConfigSources(sources ...config.Source) Option {
	return func(value *options) error { value.sources = append(value.sources, sources...); return nil }
}
func WithObserver(observer Observer) Option {
	return func(value *options) error { value.observer = observer; return nil }
}
func WithConfigWatch() Option { return func(value *options) error { value.watch = true; return nil } }
func WithStartupTimeout(timeout time.Duration) Option {
	return durationOption("startup", timeout, func(value *options) { value.startupTimeout = timeout })
}
func WithShutdownTimeout(timeout time.Duration) Option {
	return durationOption("shutdown", timeout, func(value *options) { value.shutdownTimeout = timeout })
}
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

type Plan struct {
	compiled *compiled.Plan
	graph    di.Graph
	loaded   config.Snapshot
}

func (p *Plan) DependencyGraph() di.Graph {
	if p == nil {
		return di.Graph{}
	}
	return di.Graph{Nodes: append([]di.Node(nil), p.graph.Nodes...), Edges: append([]di.Edge(nil), p.graph.Edges...)}
}

type Application struct {
	stateValue atomic.Uint32
	sequence   atomic.Uint64
	plan       *compiled.Plan
	instances  []compiled.Instance
	snapshot   config.Snapshot
	options    options
	runStarted atomic.Bool
}

func (a *Application) State() State { return State(a.stateValue.Load()) }
