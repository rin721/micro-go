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

	"github.com/rin721/micro-go/types/capability/logging"
)

// ChangeResult 描述 Adapter 自身能否原地应用新配置。
// 它不引用 Kernel Reload 类型，因此日志实现可以独立于应用运行时复用。
type ChangeResult uint8

const (
	// ChangeApplied 表示配置已经安全地原地应用。
	ChangeApplied ChangeResult = iota
	// ChangeRestartRequired 表示 Handler 或输出资源必须通过重建替换。
	ChangeRestartRequired
)

// Config 是 Slog Adapter 拥有的强类型配置。
// Level 可原地更新；Output 和 JSON 会改变 Handler 或资源，需要重启。
type Config struct {
	// Level 是 debug、info、warn 或 error，并支持原地 Reload。
	Level string `yaml:"level" json:"level" validate:"required,oneof=debug info warn error"`
	// Output 是 stdout、stderr 或由 Adapter 打开的文件路径。
	Output string `yaml:"output" json:"output" validate:"required"`
	// JSON 选择 JSON Handler；false 使用文本 Handler。
	JSON bool `yaml:"json" json:"json"`
}

type owner struct {
	closer   io.Closer
	once     sync.Once
	closeErr error
}

// Logger 使用标准库 slog 实现项目日志契约，并共享级别、锁和输出所有权。
type Logger struct {
	logger *stdslog.Logger
	level  *stdslog.LevelVar
	config Config
	owner  *owner
	mu     *sync.Mutex
}

// New 创建 Handler 和 Logger；只有 Adapter 自己打开的文件会登记为 closer。
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

// Debug 写入 Debug 级别结构化日志。
func (l *Logger) Debug(ctx context.Context, message string, fields ...logging.Field) {
	l.logger.LogAttrs(ctx, stdslog.LevelDebug, message, attrs(fields)...)
}

// Info 写入 Info 级别结构化日志。
func (l *Logger) Info(ctx context.Context, message string, fields ...logging.Field) {
	l.logger.LogAttrs(ctx, stdslog.LevelInfo, message, attrs(fields)...)
}

// Warn 写入 Warn 级别结构化日志。
func (l *Logger) Warn(ctx context.Context, message string, fields ...logging.Field) {
	l.logger.LogAttrs(ctx, stdslog.LevelWarn, message, attrs(fields)...)
}

// Error 写入 Error 级别结构化日志。
func (l *Logger) Error(ctx context.Context, message string, fields ...logging.Field) {
	l.logger.LogAttrs(ctx, stdslog.LevelError, message, attrs(fields)...)
}

// With 返回带固定字段的派生 Logger，并共享底层输出资源。
func (l *Logger) With(fields ...logging.Field) logging.Logger {
	copy := *l
	values := make([]any, 0, len(fields))
	for _, field := range fields {
		values = append(values, stdslog.Any(field.Key, field.Value))
	}
	copy.logger = l.logger.With(values...)
	return &copy
}

// Named 使用 slog Group 表达命名空间，而不把 slog 类型暴露给调用方。
func (l *Logger) Named(name string) logging.Logger {
	copy := *l
	copy.logger = l.logger.WithGroup(name)
	return &copy
}

// Close 幂等关闭 Adapter 自己打开的文件；标准输出和错误输出不归它所有。
func (l *Logger) Close(context.Context) error {
	l.owner.once.Do(func() {
		if l.owner.closer != nil {
			l.owner.closeErr = l.owner.closer.Close()
		}
	})
	return l.owner.closeErr
}

// Apply 通过 LevelVar 并发安全地更新级别；Handler 或输出变化要求组合根重建应用。
func (l *Logger) Apply(candidate Config) (ChangeResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if candidate.Output != l.config.Output || candidate.JSON != l.config.JSON {
		return ChangeRestartRequired, nil
	}
	level, err := parseLevel(candidate.Level)
	if err != nil {
		return ChangeApplied, err
	}
	l.level.Set(level)
	l.config = candidate
	return ChangeApplied, nil
}

func attrs(fields []logging.Field) []stdslog.Attr {
	// slog.Attr 在 Adapter 边界内临时生成，业务层只感知 logging.Field。
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

// 编译期断言只约束公共日志能力；生命周期和 Reload 由组合根桥接。
var _ logging.Logger = (*Logger)(nil)
