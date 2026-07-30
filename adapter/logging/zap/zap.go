// Package zap 使用 Uber Zap 实现项目日志契约。
package zap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/rin721/micro-go/capability/logging"
	"github.com/rin721/micro-go/kernel/config"
	"github.com/rin721/micro-go/kernel/lifecycle"
	"github.com/rin721/micro-go/kernel/module"
	"github.com/rin721/micro-go/kernel/reload"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	Level       string `yaml:"level" json:"level" validate:"required,oneof=debug info warn error"`
	Development bool   `yaml:"development" json:"development"`
	Output      string `yaml:"output" json:"output" validate:"required"`
}

type Logger struct {
	logger *zap.Logger
	level  zap.AtomicLevel
	config Config
	mu     *sync.Mutex
	owner  *owner
}

type owner struct {
	once   sync.Once
	closer io.Closer
	err    error
}

func New(cfg Config) (*Logger, error) {
	level, err := zapcore.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		return nil, fmt.Errorf("parse zap level: %w", err)
	}
	atomicLevel := zap.NewAtomicLevelAt(level)
	writer, closer, err := zapOutput(cfg.Output)
	if err != nil {
		return nil, err
	}
	encoderConfig := zap.NewProductionEncoderConfig()
	encoder := zapcore.NewJSONEncoder(encoderConfig)
	if cfg.Development {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}
	logger := zap.New(zapcore.NewCore(encoder, zapcore.AddSync(writer), atomicLevel))
	return &Logger{logger: logger, level: atomicLevel, config: cfg, mu: &sync.Mutex{}, owner: &owner{closer: closer}}, nil
}

func (l *Logger) Debug(_ context.Context, message string, fields ...logging.Field) {
	l.logger.Debug(message, convert(fields)...)
}
func (l *Logger) Info(_ context.Context, message string, fields ...logging.Field) {
	l.logger.Info(message, convert(fields)...)
}
func (l *Logger) Warn(_ context.Context, message string, fields ...logging.Field) {
	l.logger.Warn(message, convert(fields)...)
}
func (l *Logger) Error(_ context.Context, message string, fields ...logging.Field) {
	l.logger.Error(message, convert(fields)...)
}
func (l *Logger) With(fields ...logging.Field) logging.Logger {
	copy := *l
	copy.logger = l.logger.With(convert(fields)...)
	return &copy
}
func (l *Logger) Named(name string) logging.Logger {
	copy := *l
	copy.logger = l.logger.Named(name)
	return &copy
}

func (l *Logger) Close(context.Context) (result error) {
	l.owner.once.Do(func() {
		if err := l.logger.Sync(); err != nil && !isStandardStreamSync(err) {
			l.owner.err = fmt.Errorf("sync zap logger: %w", err)
		}
		if l.owner.closer != nil {
			l.owner.err = errors.Join(l.owner.err, l.owner.closer.Close())
		}
	})
	return l.owner.err
}

func zapOutput(value string) (io.Writer, io.Closer, error) {
	switch value {
	case "stdout":
		return os.Stdout, nil, nil
	case "stderr":
		return os.Stderr, nil, nil
	default:
		file, err := os.OpenFile(value, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, nil, fmt.Errorf("open zap output: %w", err)
		}
		return file, file, nil
	}
}

func (l *Logger) Reload(_ context.Context, snapshot config.Snapshot) (reload.Result, error) {
	candidate, err := config.Value[Config](snapshot)
	if err != nil {
		return reload.Ignored, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if candidate.Output != l.config.Output || candidate.Development != l.config.Development {
		return reload.RestartRequired, nil
	}
	level, err := zapcore.ParseLevel(strings.ToLower(candidate.Level))
	if err != nil {
		return reload.Ignored, fmt.Errorf("parse zap level: %w", err)
	}
	l.level.SetLevel(level)
	l.config = candidate
	return reload.Applied, nil
}

func convert(fields []logging.Field) []zap.Field {
	result := make([]zap.Field, 0, len(fields))
	for _, field := range fields {
		result = append(result, zap.Any(field.Key, field.Value))
	}
	return result
}

func isStandardStreamSync(err error) bool {
	message := strings.ToLower(err.Error())
	return errors.Is(err, syscall.EINVAL) || strings.Contains(message, "invalid argument") || strings.Contains(message, "inappropriate ioctl")
}

type Module struct{}

func (Module) Name() string { return "logging-zap" }
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
