// 本文件声明默认 application Module，并提供受 Runtime 监督的最小进程 Runner。
package bootstrap

import (
	"context"

	"github.com/rin721/micro-go/internal/kernel/module"
	"github.com/rin721/micro-go/types/capability/clock"
	"github.com/rin721/micro-go/types/capability/idgen"
	"github.com/rin721/micro-go/types/capability/logging"
)

// applicationConfig 是 application Module 独占的强类型配置。
type applicationConfig struct {
	// Name 是日志和实例诊断使用的非空应用标识。
	Name string `yaml:"name" json:"name" validate:"required"`
}

// applicationModule 声明默认业务 Runner，并通过构造函数接收已导出的 Capability。
type applicationModule struct{}

// Name 返回依赖图中的稳定模块名。
func (applicationModule) Name() string { return "application.process" }

// Register 声明应用配置和 process Provider，不导出具体 process 类型。
func (applicationModule) Register(registry module.Registry) error {
	// 配置声明先于 Provider，后者可以按普通参数接收 applicationConfig。
	if err := module.Config[applicationConfig](registry, "application"); err != nil {
		return err
	}
	// process 是当前模块最终工作负载，由 Runtime 通过 Runner 接口发现。
	return module.Provide(registry, newProcess)
}

// process 是默认长期任务，只依赖项目 Capability，不直接创建 Adapter。
type process struct {
	// name 是已经验证的应用标识。
	name string
	// logger 是附加 app 命名空间的业务 Logger。
	logger logging.Logger
	// clock 提供可替换当前时间。
	clock clock.Clock
	// ids 为每次进程启动生成实例标识。
	ids idgen.Generator
}

// newProcess 通过显式构造参数接收配置和全部 Capability。
func newProcess(cfg applicationConfig, logger logging.Logger, appClock clock.Clock, ids idgen.Generator) *process {
	// 只派生日志命名空间，底层资源所有权仍属于日志 Module。
	return &process{name: cfg.Name, logger: logger.Named("app"), clock: appClock, ids: ids}
}

// Run 表示由 Runtime 监督的主业务循环；退出只由根 Context 或真实业务错误驱动。
func (p *process) Run(ctx context.Context) error {
	// 启动事件同时记录应用名、单次实例 ID 和 Capability 提供的当前时间。
	p.logger.Info(ctx, "application started", logging.String("application", p.name), logging.String("instance_id", p.ids.New()), logging.Time("time", p.clock.Now()))
	// 默认脚手架没有业务循环，持续等待 Runtime 取消根 Context。
	<-ctx.Done()
	// 返回取消原因让 Runtime 识别为协作退出，而不是意外 nil 返回。
	return ctx.Err()
}
