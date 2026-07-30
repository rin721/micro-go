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

// Config 是 Zap Adapter 拥有的强类型配置。
// Level 可以原地更新；Development 和 Output 会改变编码器或资源，必须通过重启替换。
type Config struct {
	// Level 是 debug、info、warn 或 error，并支持原地 Reload。
	Level string `yaml:"level" json:"level" validate:"required,oneof=debug info warn error"`
	// Development 选择开发 Console Encoder；false 使用生产 JSON Encoder。
	Development bool `yaml:"development" json:"development"`
	// Output 是 stdout、stderr 或由 Adapter 打开的文件路径。
	Output string `yaml:"output" json:"output" validate:"required"`
}

// Logger 将 capability Logger 转换为 Zap 强类型 Logger，并实现关闭与配置重载。
// 派生 Logger 复制该结构，但共享 mu、AtomicLevel 和 owner。
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

// New 根据配置创建 Zap Core，并明确取得文件输出的关闭所有权。
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

// Debug 写入 Debug 级别结构化日志。
func (l *Logger) Debug(_ context.Context, message string, fields ...logging.Field) {
	l.logger.Debug(message, convert(fields)...)
}

// Info 写入 Info 级别结构化日志。
func (l *Logger) Info(_ context.Context, message string, fields ...logging.Field) {
	l.logger.Info(message, convert(fields)...)
}

// Warn 写入 Warn 级别结构化日志。
func (l *Logger) Warn(_ context.Context, message string, fields ...logging.Field) {
	l.logger.Warn(message, convert(fields)...)
}

// Error 写入 Error 级别结构化日志。
func (l *Logger) Error(_ context.Context, message string, fields ...logging.Field) {
	l.logger.Error(message, convert(fields)...)
}

// With 返回携带固定字段的派生 Logger；底层输出资源仍由共享 owner 只关闭一次。
func (l *Logger) With(fields ...logging.Field) logging.Logger {
	copy := *l
	copy.logger = l.logger.With(convert(fields)...)
	return &copy
}

// Named 返回带 Zap Logger 名称的派生 Logger。
func (l *Logger) Named(name string) logging.Logger {
	copy := *l
	copy.logger = l.logger.Named(name)
	return &copy
}

// Close 刷新缓冲并关闭自有文件，sync.Once 保证所有派生 Logger 共享幂等关闭语义。
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
	// 标准流由进程拥有，Adapter 只关闭自己打开的文件。
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

// Reload 只原子更新日志级别；输出或编码器变化需要重新创建 Core，因此请求重启。
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
	// 第三方 zap.Field 只在本包内部产生，公共 Logger 契约始终使用项目 Field。
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

// Module 注册 Zap Logger 配置、实现与公共日志契约。
type Module struct{}

// Name 返回稳定模块名。
func (Module) Name() string { return "logging-zap" }

// Register 按 Config、Provider、Binding、Export 顺序声明完整日志模块。
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

// 编译期断言明确 Logger 同时参与公共能力、资源关闭和配置重载。
var _ logging.Logger = (*Logger)(nil)
var _ lifecycle.Closer = (*Logger)(nil)
var _ reload.Reloader = (*Logger)(nil)
