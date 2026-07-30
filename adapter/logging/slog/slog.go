// Package slog 使用标准库 log/slog 实现项目日志契约。
package slog

import (
	"context"
	"fmt"
	"io"
	stdslog "log/slog"
	"os"
	"strings"
	"sync"

	"github.com/rin721/micro-go/capability/logging"
	"github.com/rin721/micro-go/kernel/config"
	"github.com/rin721/micro-go/kernel/lifecycle"
	"github.com/rin721/micro-go/kernel/module"
	"github.com/rin721/micro-go/kernel/reload"
)

type Config struct {
	Level  string `yaml:"level" json:"level" validate:"required,oneof=debug info warn error"`
	Output string `yaml:"output" json:"output" validate:"required"`
	JSON   bool   `yaml:"json" json:"json"`
}

type owner struct {
	closer   io.Closer
	once     sync.Once
	closeErr error
}
type Logger struct {
	logger *stdslog.Logger
	level  *stdslog.LevelVar
	config Config
	owner  *owner
	mu     *sync.Mutex
}

func New(cfg Config) (*Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	levelVar := &stdslog.LevelVar{}
	levelVar.Set(level)
	writer, closer, err := output(cfg.Output)
	if err != nil {
		return nil, err
	}
	options := &stdslog.HandlerOptions{Level: levelVar}
	var handler stdslog.Handler = stdslog.NewTextHandler(writer, options)
	if cfg.JSON {
		handler = stdslog.NewJSONHandler(writer, options)
	}
	return &Logger{logger: stdslog.New(handler), level: levelVar, config: cfg, owner: &owner{closer: closer}, mu: &sync.Mutex{}}, nil
}

func (l *Logger) Debug(ctx context.Context, message string, fields ...logging.Field) {
	l.logger.LogAttrs(ctx, stdslog.LevelDebug, message, attrs(fields)...)
}
func (l *Logger) Info(ctx context.Context, message string, fields ...logging.Field) {
	l.logger.LogAttrs(ctx, stdslog.LevelInfo, message, attrs(fields)...)
}
func (l *Logger) Warn(ctx context.Context, message string, fields ...logging.Field) {
	l.logger.LogAttrs(ctx, stdslog.LevelWarn, message, attrs(fields)...)
}
func (l *Logger) Error(ctx context.Context, message string, fields ...logging.Field) {
	l.logger.LogAttrs(ctx, stdslog.LevelError, message, attrs(fields)...)
}
func (l *Logger) With(fields ...logging.Field) logging.Logger {
	copy := *l
	values := make([]any, 0, len(fields))
	for _, field := range fields {
		values = append(values, stdslog.Any(field.Key, field.Value))
	}
	copy.logger = l.logger.With(values...)
	return &copy
}
func (l *Logger) Named(name string) logging.Logger {
	copy := *l
	copy.logger = l.logger.WithGroup(name)
	return &copy
}

func (l *Logger) Close(context.Context) error {
	l.owner.once.Do(func() {
		if l.owner.closer != nil {
			l.owner.closeErr = l.owner.closer.Close()
		}
	})
	return l.owner.closeErr
}

func (l *Logger) Reload(_ context.Context, snapshot config.Snapshot) (reload.Result, error) {
	candidate, err := config.Value[Config](snapshot)
	if err != nil {
		return reload.Ignored, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if candidate.Output != l.config.Output || candidate.JSON != l.config.JSON {
		return reload.RestartRequired, nil
	}
	level, err := parseLevel(candidate.Level)
	if err != nil {
		return reload.Ignored, err
	}
	l.level.Set(level)
	l.config = candidate
	return reload.Applied, nil
}

func attrs(fields []logging.Field) []stdslog.Attr {
	result := make([]stdslog.Attr, 0, len(fields))
	for _, field := range fields {
		result = append(result, stdslog.Any(field.Key, field.Value))
	}
	return result
}
func parseLevel(value string) (stdslog.Level, error) {
	switch strings.ToLower(value) {
	case "debug":
		return stdslog.LevelDebug, nil
	case "info":
		return stdslog.LevelInfo, nil
	case "warn":
		return stdslog.LevelWarn, nil
	case "error":
		return stdslog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported slog level %q", value)
	}
}
func output(value string) (io.Writer, io.Closer, error) {
	switch value {
	case "stdout":
		return os.Stdout, nil, nil
	case "stderr":
		return os.Stderr, nil, nil
	default:
		file, err := os.OpenFile(value, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, nil, fmt.Errorf("open slog output: %w", err)
		}
		return file, file, nil
	}
}

type Module struct{}

func (Module) Name() string { return "logging-slog" }
func (Module) Register(registry module.Registry) error {
	if err := module.Config[Config](registry, "logging"); err != nil {
		return err
	}
	if err := module.Provide(registry, New); err != nil {
		return err
	}
	if err := module.Bind[logging.Logger, *Logger](registry); err != nil {
		return err
	}
	return module.Export[logging.Logger](registry)
}

var _ logging.Logger = (*Logger)(nil)
var _ lifecycle.Closer = (*Logger)(nil)
var _ reload.Reloader = (*Logger)(nil)
