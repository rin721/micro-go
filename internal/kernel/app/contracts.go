// Package app 定义单进程应用运行时必须遵守的状态、事件与执行契约。
//
// 该包只表达 Kernel 语义，不负责选择 Dig、Koanf 或具体 Runtime。把契约放在
// internal 下可以阻止项目重新演变为需要长期兼容的外部框架 API，同时让默认实现
// 和组合根仍能围绕同一套稳定语义协作。
package app

import (
	"context"
	"errors"
	"time"

	"github.com/rin721/micro-go/internal/kernel/di"
	"github.com/rin721/micro-go/internal/kernel/diagnostic"
)

// State 是 Application 的单向运行状态。
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
	// ErrAlreadyRun 表示同一个 Application 被重复运行。
	ErrAlreadyRun = errors.New("application can only run once")
	// ErrRestartRequired 表示候选配置有效，但至少一个组件要求重启应用。
	ErrRestartRequired = errors.New("configuration change requires application restart")
)

// EventKind 对 Observer 可见的事件进行稳定分类。
type EventKind string

const (
	// StateChanged 表示 Application 状态发生变化。
	StateChanged EventKind = "state.changed"
	// ConfigurationLoad 表示初始配置或候选配置已经成功加载。
	ConfigurationLoad EventKind = "config.loaded"
	// ConfigurationFail 表示候选配置无效，或候选交给组件应用时失败。
	ConfigurationFail EventKind = "config.failed"
	// GraphCompiled 表示依赖图编译完成。
	GraphCompiled EventKind = "graph.compiled"
	// ComponentEvent 表示某个组件完成一次生命周期调用，Err 保存本次结果。
	ComponentEvent EventKind = "component.lifecycle"
	// RunnerFailed 表示受监督 Runner 返回错误。
	RunnerFailed EventKind = "runner.failed"
)

// Event 是同步 Observer 收到的项目自有只读事件。
type Event struct {
	// Sequence 是单个 Application 内的递增序号。
	Sequence uint64
	// Time 是事件生成的 UTC 时间。
	Time time.Time
	// Kind 是事件稳定分类。
	Kind EventKind
	// State 是事件发生时或转换后的应用状态。
	State State
	// Phase 是对应的 Kernel 执行阶段。
	Phase diagnostic.Phase
	// Module 在组件事件中标识所属模块。
	Module string
	// Component 在组件事件中标识具体类型。
	Component string
	// Err 是已经过 Kernel 边界归一化的错误。
	Err error
}

// Observer 同步观察 Kernel 事件；实现必须快速返回且不得修改运行时。
type Observer interface{ Observe(Event) }

// Plan 只公开稳定依赖图，不允许调用方在运行期解析组件实例。
type Plan interface{ DependencyGraph() di.Graph }

// Application 是组合根可驱动的一次性单进程运行时。
type Application interface {
	Run(context.Context) error
	State() State
}
