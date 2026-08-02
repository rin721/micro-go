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

	"github.com/rin721/micro-go/types/capability/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ChangeResult 描述 Zap Core 是否可以在不重建资源的前提下应用配置。
type ChangeResult uint8

const (
	// ChangeApplied 表示配置已经安全地原地应用。
	ChangeApplied ChangeResult = iota
	// ChangeRestartRequired 表示编码器或输出资源需要重新构造。
	ChangeRestartRequired
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
	// logger 是当前派生实例专属的 Zap Logger 值。
	logger *zap.Logger
	// level 由所有派生实例共享，支持原子更新过滤级别。
	level zap.AtomicLevel
	// config 保存上次成功应用的完整配置，用于判断是否需要重启。
	config Config
	// mu 串行化 config 比较与 AtomicLevel 更新。
	mu *sync.Mutex
	// owner 在所有派生实例之间共享文件关闭和最终错误。
	owner *owner
}

// owner 集中执行一次底层同步和文件关闭，并缓存最终结果。
type owner struct {
	// once 保证所有派生 Logger 中只有首次 Close 执行资源释放。
	once sync.Once
	// closer 仅在 Adapter 自己打开文件时非 nil。
	closer io.Closer
	// err 保存同步错误与关闭错误的完整合并结果。
	err error
}

// New 根据配置创建 Zap Core，并明确取得文件输出的关闭所有权。
func New(cfg Config) (*Logger, error) {
	// 级别解析先于资源创建，非法配置不会留下需要关闭的文件。
	level, err := zapcore.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		return nil, fmt.Errorf("parse zap level: %w", err)
	}
	// AtomicLevel 是派生 Logger 共享的原地 Reload 控制点。
	atomicLevel := zap.NewAtomicLevelAt(level)
	// output 同时返回 Writer 和可选关闭责任，标准流不归 Adapter 所有。
	writer, closer, err := zapOutput(cfg.Output)
	if err != nil {
		return nil, err
	}
	// 默认生产 JSON；Development 显式选择更适合本地阅读的 Console Encoder。
	encoderConfig := zap.NewProductionEncoderConfig()
	encoder := zapcore.NewJSONEncoder(encoderConfig)
	if cfg.Development {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}
	// Core 固定 Encoder 和 Writer，只有级别可以在当前实例上原子调整。
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
	// 浅复制保留共享 level、mu 和 owner，只替换派生后的 Zap Logger 值。
	copy := *l
	copy.logger = l.logger.With(convert(fields)...)
	return &copy
}

// Named 返回带 Zap Logger 名称的派生 Logger。
func (l *Logger) Named(name string) logging.Logger {
	// 命名派生不创建新输出资源，因此继续共享关闭所有权。
	copy := *l
	copy.logger = l.logger.Named(name)
	return &copy
}

// Close 刷新缓冲并关闭自有文件，sync.Once 保证所有派生 Logger 共享幂等关闭语义。
func (l *Logger) Close(context.Context) (result error) {
	// once 覆盖同步和关闭的完整事务，所有调用者读取同一缓存错误。
	l.owner.once.Do(func() {
		// Windows 和部分标准流不支持 Sync；仅忽略已识别的标准流错误。
		if err := l.logger.Sync(); err != nil && !isStandardStreamSync(err) {
			l.owner.err = fmt.Errorf("sync zap logger: %w", err)
		}
		// 只有文件输出具有 closer，并与可能的 Sync 错误一同保留。
		if l.owner.closer != nil {
			l.owner.err = errors.Join(l.owner.err, l.owner.closer.Close())
		}
	})
	return l.owner.err
}

// zapOutput 解析标准流名称，或以受限权限打开由 Adapter 拥有的追加文件。
func zapOutput(value string) (io.Writer, io.Closer, error) {
	// 标准流由进程拥有，Adapter 只关闭自己打开的文件。
	switch value {
	case "stdout":
		return os.Stdout, nil, nil
	case "stderr":
		return os.Stderr, nil, nil
	default:
		// 追加写避免启动时截断已有日志；0600 防止新文件向其他用户泄露诊断。
		file, err := os.OpenFile(value, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, nil, fmt.Errorf("open zap output: %w", err)
		}
		return file, file, nil
	}
}

// Apply 只原子更新日志级别；输出或编码器变化需要由组合根重建应用。
func (l *Logger) Apply(candidate Config) (ChangeResult, error) {
	// 比较和更新必须共享一把锁，避免并发候选覆盖 config 与 level 的对应关系。
	l.mu.Lock()
	defer l.mu.Unlock()
	if candidate.Output != l.config.Output || candidate.Development != l.config.Development {
		return ChangeRestartRequired, nil
	}
	// 先解析候选级别；失败时当前 AtomicLevel 和 config 都保持不变。
	level, err := zapcore.ParseLevel(strings.ToLower(candidate.Level))
	if err != nil {
		return ChangeApplied, fmt.Errorf("parse zap level: %w", err)
	}
	// AtomicLevel 对已有 Core 原子生效，然后记录同一份已应用配置。
	l.level.SetLevel(level)
	l.config = candidate
	return ChangeApplied, nil
}

// convert 把项目字段切片转换为 Zap 字段切片。
func convert(fields []logging.Field) []zap.Field {
	// 第三方 zap.Field 只在本包内部产生，公共 Logger 契约始终使用项目 Field。
	result := make([]zap.Field, 0, len(fields))
	for _, field := range fields {
		result = append(result, zap.Any(field.Key, field.Value))
	}
	return result
}

// isStandardStreamSync 识别不同平台对 stdout/stderr Sync 返回的无害错误。
func isStandardStreamSync(err error) bool {
	// 同时检查 syscall 类型和常见平台文本，其他 I/O 错误仍完整上报。
	message := strings.ToLower(err.Error())
	return errors.Is(err, syscall.EINVAL) || strings.Contains(message, "invalid argument") || strings.Contains(message, "inappropriate ioctl")
}

// 编译期断言只约束公共日志能力；生命周期和 Reload 由组合根桥接。
var _ logging.Logger = (*Logger)(nil)
