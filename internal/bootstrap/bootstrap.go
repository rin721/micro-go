// Package bootstrap 是单进程应用唯一的组合根。
//
// 业务组件只依赖 types/capability 契约；具体 Adapter、Kernel 默认实现、配置优先级
// 和模块导出关系全部在这里一次性决定。集中装配可以避免业务包自行创建第二套客户端，
// 也让未来替换 Dig、Koanf 或日志实现时只修改这一处技术选择。
package bootstrap

import (
	"context"
	"io"
	"os"
	"time"

	configwatcher "github.com/rin721/micro-go/internal/adapter/kernel/config/fsnotify"
	koanfadapter "github.com/rin721/micro-go/internal/adapter/kernel/config/koanf"
	configsource "github.com/rin721/micro-go/internal/adapter/kernel/config/source"
	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiler"
	digadapter "github.com/rin721/micro-go/internal/adapter/kernel/di/dig"
	registration "github.com/rin721/micro-go/internal/adapter/kernel/module"
	runtimeadapter "github.com/rin721/micro-go/internal/adapter/kernel/runtime"
)

const (
	defaultConfigFile      = "config/app.yaml"
	configFileEnvironment  = "APP_CONFIG_FILE"
	startupTimeout         = 15 * time.Second
	shutdownTimeout        = 15 * time.Second
	reloadTimeout          = 15 * time.Second
	reloadDebounce         = 200 * time.Millisecond
	defaultApplicationName = "micro-go"
)

// Run 构造并驱动当前应用，直到 Runner 结束、配置要求重启或根 Context 被取消。
// 函数返回前 Runtime 已经完成 Stop、等待 Runner 和 Close，因此 cmd/app 不拥有组件资源。
func Run(ctx context.Context) error {
	return run(ctx, os.Stderr)
}

func run(ctx context.Context, diagnosticWriter io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if diagnosticWriter == nil {
		diagnosticWriter = io.Discard
	}
	fileSource, err := configsource.FromFile(configFile())
	if err != nil {
		return err
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
			fileSource,
			configsource.FromEnvironment("APP", configFileEnvironment),
		),
		runtimeadapter.WithConfigWatch(),
		runtimeadapter.WithObserver(newRuntimeObserver(diagnosticWriter)),
		runtimeadapter.WithStartupTimeout(startupTimeout),
		runtimeadapter.WithShutdownTimeout(shutdownTimeout),
		runtimeadapter.WithReloadTimeout(reloadTimeout),
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
		"application": map[string]any{"name": defaultApplicationName},
		"logging":     map[string]any{"level": "info", "output": "stdout", "json": false},
	}
}
