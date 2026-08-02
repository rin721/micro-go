// Package bootstrap 是单进程应用唯一的组合根。
//
// 业务组件只依赖 types/capability 契约；具体 Adapter、Kernel 默认实现、配置优先级
// 和模块导出关系全部在这里一次性决定。集中装配可以避免业务包自行创建第二套客户端，
// 也让未来替换 Dig、Koanf 或日志实现时只修改这一处技术选择。
package bootstrap

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	configwatcher "github.com/rin721/micro-go/internal/adapter/kernel/config/fsnotify"
	koanfadapter "github.com/rin721/micro-go/internal/adapter/kernel/config/koanf"
	configsource "github.com/rin721/micro-go/internal/adapter/kernel/config/source"
	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiler"
	digadapter "github.com/rin721/micro-go/internal/adapter/kernel/di/dig"
	kernelslog "github.com/rin721/micro-go/internal/adapter/kernel/logging/slog"
	registration "github.com/rin721/micro-go/internal/adapter/kernel/module"
	runtimeadapter "github.com/rin721/micro-go/internal/adapter/kernel/runtime"
	"github.com/rin721/micro-go/internal/kernel/diagnostic"
	capabilitylogging "github.com/rin721/micro-go/types/capability/logging"
)

const (
	// defaultConfigFile 是未设置进程覆盖变量时读取的仓库相对配置路径。
	defaultConfigFile = "config/app.yaml"
	// configFileEnvironment 允许部署环境选择另一份配置文件。
	configFileEnvironment = "APP_CONFIG_FILE"
	// startupTimeout 限制默认应用的 Prepare 与 Start 总时间。
	startupTimeout = 15 * time.Second
	// shutdownTimeout 限制 Stop、Runner 等待和 Close 总时间。
	shutdownTimeout = 15 * time.Second
	// reloadTimeout 限制一次完整候选配置应用。
	reloadTimeout = 15 * time.Second
	// reloadDebounce 合并编辑器一次保存产生的重复文件事件。
	reloadDebounce = 200 * time.Millisecond
	// defaultApplicationName 是初始化脚本会替换的模板应用标识。
	defaultApplicationName = "micro-go"
)

// Run 构造并驱动当前应用，直到 Runner 结束、配置要求重启或根 Context 被取消。
// 函数返回前 Runtime 已经完成 Stop、等待 Runner 和 Close，因此 cmd/app 不拥有组件资源。
func Run(ctx context.Context) error {
	// 生产入口固定把早期 Kernel 诊断写到标准错误。
	return run(ctx, os.Stderr)
}

// run 允许测试注入诊断 Writer，同时完成与生产 Run 完全相同的装配和生命周期。
func run(ctx context.Context, diagnosticWriter io.Writer) (runErr error) {
	// nil Context 表示调用方没有主动取消来源，统一替换为 Background。
	if ctx == nil {
		ctx = context.Background()
	}
	// nil Writer 转为空输出而不是让 Logger 构造或后续写入 panic。
	if diagnosticWriter == nil {
		diagnosticWriter = io.Discard
	}
	// Kernel Logger 必须在读取配置前创建，确保最早失败也有诊断出口。
	kernelLogger := kernelslog.New(diagnosticWriter)
	// Logger 关闭错误与应用错误合并，不能被命名返回值覆盖。
	defer func() { runErr = errors.Join(runErr, kernelLogger.Close()) }()

	// 文件 Source 在 Build 前解析绝对路径，失败经基线 Logger 记录一次。
	fileSource, err := configsource.FromFile(configFile())
	if err != nil {
		kernelLogger.Error(ctx, "bootstrap failed", capabilitylogging.String("phase", string(diagnostic.ConfigLoad)), capabilitylogging.String("error", diagnostic.Redact(err.Error())))
		return err
	}
	// 所有 Runtime 部件在唯一组合根显式选择，Runtime 内部不自行创建第三方实现。
	runtime, err := runtimeadapter.New(runtimeadapter.Dependencies{
		Logger:      kernelLogger,
		Collector:   registration.NewCollector(),
		Compiler:    compiler.New(),
		Loader:      koanfadapter.New(),
		Constructor: digadapter.New(),
		Watcher:     configwatcher.New(),
	})
	if err != nil {
		kernelLogger.Error(ctx, "bootstrap failed", capabilitylogging.String("phase", string(diagnostic.Construct)), capabilitylogging.String("error", diagnostic.Redact(err.Error())))
		return err
	}

	// 模块顺序、配置覆盖顺序、监听策略和所有时间边界在一次 Build 调用中冻结。
	application, err := runtime.Build(ctx,
		runtimeadapter.WithModules(newLoggingModule(kernelLogger), clockModule{}, idModule{}, applicationModule{}),
		runtimeadapter.WithConfigSources(
			configsource.FromValues(defaultValues()),
			fileSource,
			configsource.FromEnvironment("APP", configFileEnvironment),
		),
		runtimeadapter.WithConfigWatch(),
		runtimeadapter.WithStartupTimeout(startupTimeout),
		runtimeadapter.WithShutdownTimeout(shutdownTimeout),
		runtimeadapter.WithReloadTimeout(reloadTimeout),
		runtimeadapter.WithReloadDebounce(reloadDebounce),
	)
	if err != nil {
		return err
	}
	// Build 成功后由 Application 独占驱动全部组件，返回前完成统一关停。
	return application.Run(ctx)
}

// configFile 返回部署环境覆盖路径，未设置时使用仓库默认配置。
func configFile() string {
	// 空环境变量不覆盖默认值，避免将空路径伪装成有效选择。
	if value := os.Getenv(configFileEnvironment); value != "" {
		return value
	}
	return defaultConfigFile
}

// defaultValues 返回最低优先级的完整默认配置树。
func defaultValues() map[string]any {
	// 每次调用创建新 map，Loader 深复制后不会与其他 Build 共享可变状态。
	return map[string]any{
		"application": map[string]any{"name": defaultApplicationName},
		"logging":     map[string]any{"level": "info", "output": "stdout", "json": false},
	}
}
