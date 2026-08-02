package bootstrap

import (
	"context"

	"github.com/rin721/micro-go/internal/kernel/module"
	"github.com/rin721/micro-go/types/capability/clock"
	"github.com/rin721/micro-go/types/capability/idgen"
	"github.com/rin721/micro-go/types/capability/logging"
)

type applicationConfig struct {
	Name string `yaml:"name" json:"name" validate:"required"`
}

// applicationModule 声明默认业务 Runner，并通过构造函数接收已导出的 Capability。
type applicationModule struct{}

func (applicationModule) Name() string { return "application.process" }

func (applicationModule) Register(registry module.Registry) error {
	if err := module.Config[applicationConfig](registry, "application"); err != nil {
		return err
	}
	return module.Provide(registry, newProcess)
}

type process struct {
	name   string
	logger logging.Logger
	clock  clock.Clock
	ids    idgen.Generator
}

func newProcess(cfg applicationConfig, logger logging.Logger, appClock clock.Clock, ids idgen.Generator) *process {
	return &process{name: cfg.Name, logger: logger.Named("app"), clock: appClock, ids: ids}
}

// Run 表示由 Runtime 监督的主业务循环；退出只由根 Context 或真实业务错误驱动。
func (p *process) Run(ctx context.Context) error {
	p.logger.Info(ctx, "application started", logging.String("application", p.name), logging.String("instance_id", p.ids.New()), logging.Time("time", p.clock.Now()))
	<-ctx.Done()
	return ctx.Err()
}
