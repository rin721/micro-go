// Package bootstrap 是单进程应用唯一的组合根。
//
// 业务组件只依赖 types/capability 契约；具体 Adapter、Kernel 默认实现、配置优先级
// 和模块导出关系全部在这里一次性决定。集中装配可以避免业务包自行创建第二套客户端，
// 也让未来替换 Dig、Koanf 或日志实现时只修改这一处技术选择。
package bootstrap

import (
	"context"
	"os"
	"time"

	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/module"
	"github.com/rin721/micro-go/internal/kernel/reload"
	"github.com/rin721/micro-go/pkg/adapter/clock/system"
	uuidadapter "github.com/rin721/micro-go/pkg/adapter/idgen/uuid"
	configwatcher "github.com/rin721/micro-go/pkg/adapter/kernel/config/fsnotify"
	koanfadapter "github.com/rin721/micro-go/pkg/adapter/kernel/config/koanf"
	configsource "github.com/rin721/micro-go/pkg/adapter/kernel/config/source"
	"github.com/rin721/micro-go/pkg/adapter/kernel/di/compiler"
	digadapter "github.com/rin721/micro-go/pkg/adapter/kernel/di/dig"
	registration "github.com/rin721/micro-go/pkg/adapter/kernel/module"
	runtimeadapter "github.com/rin721/micro-go/pkg/adapter/kernel/runtime"
	slogadapter "github.com/rin721/micro-go/pkg/adapter/logging/slog"
	"github.com/rin721/micro-go/types/capability/clock"
	"github.com/rin721/micro-go/types/capability/idgen"
	"github.com/rin721/micro-go/types/capability/logging"
)

const (
	defaultConfigFile     = "config/app.yaml"
	configFileEnvironment = "APP_CONFIG_FILE"
	startupTimeout        = 15 * time.Second
	shutdownTimeout       = 15 * time.Second
	reloadDebounce        = 200 * time.Millisecond
)

// Run 构造并驱动当前应用，直到 Runner 结束、配置要求重启或根 Context 被取消。
// 函数返回前 Runtime 已经完成 Stop、等待 Runner 和 Close，因此 cmd/app 不拥有组件资源。
func Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	runtime, err := runtimeadapter.New(runtimeadapter.Dependencies{
		Collector:   registration.NewCollector(),
		Compiler:    compiler.New(),
		Loader:      koanfadapter.New(),
		Constructor: digadapter.New(),
		Watcher:     configwatcher.New(),
	})
	if err != nil {
		return err
	}

	application, err := runtime.Build(ctx,
		runtimeadapter.WithModules(loggingModule{}, clockModule{}, idModule{}, applicationModule{}),
		runtimeadapter.WithConfigSources(
			configsource.FromValues(defaultValues()),
			configsource.FromFile(configFile()),
			configsource.FromEnvironment("APP"),
		),
		runtimeadapter.WithConfigWatch(),
		runtimeadapter.WithStartupTimeout(startupTimeout),
		runtimeadapter.WithShutdownTimeout(shutdownTimeout),
		runtimeadapter.WithReloadDebounce(reloadDebounce),
	)
	if err != nil {
		return err
	}
	return application.Run(ctx)
}

func configFile() string {
	if value := os.Getenv(configFileEnvironment); value != "" {
		return value
	}
	return defaultConfigFile
}

func defaultValues() map[string]any {
	return map[string]any{
		"logging": map[string]any{"level": "info", "output": "stdout", "json": false},
	}
}

type loggingConfig struct {
	Level  string `yaml:"level" json:"level" validate:"required,oneof=debug info warn error"`
	Output string `yaml:"output" json:"output" validate:"required"`
	JSON   bool   `yaml:"json" json:"json"`
}

type managedLogger struct{ *slogadapter.Logger }

func newManagedLogger(value loggingConfig) (*managedLogger, error) {
	logger, err := slogadapter.New(slogadapter.Config{Level: value.Level, Output: value.Output, JSON: value.JSON})
	if err != nil {
		return nil, err
	}
	return &managedLogger{Logger: logger}, nil
}

// Reload 把 Kernel Snapshot 翻译为 Slog 自有配置；两侧类型不会互相穿透包边界。
func (l *managedLogger) Reload(_ context.Context, snapshot config.Snapshot) (reload.Result, error) {
	value, err := config.Value[loggingConfig](snapshot)
	if err != nil {
		return reload.Ignored, err
	}
	result, err := l.Apply(slogadapter.Config{Level: value.Level, Output: value.Output, JSON: value.JSON})
	if err != nil {
		return reload.Ignored, err
	}
	if result == slogadapter.ChangeRestartRequired {
		return reload.RestartRequired, nil
	}
	return reload.Applied, nil
}

type loggingModule struct{}

func (loggingModule) Name() string { return "foundation.logging.slog" }
func (loggingModule) Register(registry module.Registry) error {
	if err := module.Config[loggingConfig](registry, "logging"); err != nil {
		return err
	}
	if err := module.Provide(registry, newManagedLogger); err != nil {
		return err
	}
	if err := module.Bind[logging.Logger, *managedLogger](registry); err != nil {
		return err
	}
	return module.Export[logging.Logger](registry)
}

type clockModule struct{}

func (clockModule) Name() string { return "foundation.clock.system" }
func (clockModule) Register(registry module.Registry) error {
	if err := module.Provide(registry, system.New); err != nil {
		return err
	}
	if err := module.Bind[clock.Clock, *system.Clock](registry); err != nil {
		return err
	}
	return module.Export[clock.Clock](registry)
}

type idModule struct{}

func (idModule) Name() string { return "foundation.id.uuid" }
func (idModule) Register(registry module.Registry) error {
	if err := module.Provide(registry, uuidadapter.New); err != nil {
		return err
	}
	if err := module.Bind[idgen.Generator, *uuidadapter.Generator](registry); err != nil {
		return err
	}
	return module.Export[idgen.Generator](registry)
}

type applicationModule struct{}

func (applicationModule) Name() string { return "application.process" }
func (applicationModule) Register(registry module.Registry) error {
	return module.Provide(registry, newProcess)
}

type process struct {
	logger logging.Logger
	clock  clock.Clock
	ids    idgen.Generator
}

func newProcess(logger logging.Logger, appClock clock.Clock, ids idgen.Generator) *process {
	return &process{logger: logger.Named("app"), clock: appClock, ids: ids}
}

// Run 表示由 Runtime 监督的主业务循环；退出只由根 Context 或真实业务错误驱动。
func (p *process) Run(ctx context.Context) error {
	p.logger.Info(ctx, "application started", logging.String("instance_id", p.ids.New()), logging.Time("time", p.clock.Now()))
	<-ctx.Done()
	return ctx.Err()
}

var _ logging.Logger = (*managedLogger)(nil)
var _ reload.Reloader = (*managedLogger)(nil)
