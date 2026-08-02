package bootstrap

import (
	"context"

	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/module"
	"github.com/rin721/micro-go/internal/kernel/reload"
	slogadapter "github.com/rin721/micro-go/pkg/adapter/logging/slog"
	"github.com/rin721/micro-go/types/capability/logging"
)

type loggingConfig struct {
	Level  string `yaml:"level" json:"level" validate:"required,oneof=debug info warn error"`
	Output string `yaml:"output" json:"output" validate:"required"`
	JSON   bool   `yaml:"json" json:"json"`
}

// managedLogger 在组合根中桥接 Slog Adapter 的配置变化和 Kernel Reload 契约。
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

// loggingModule 选择 Slog，并将受 Kernel 管理的实现作为 Logging Capability 导出。
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

var _ logging.Logger = (*managedLogger)(nil)
var _ reload.Reloader = (*managedLogger)(nil)
