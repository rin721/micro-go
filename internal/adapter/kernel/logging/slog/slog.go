// Package slog 使用标准库 log/slog 实现 Kernel 必有日志和动态替换管理。
package slog

import (
	"context"
	"errors"
	"fmt"
	"io"
	stdslog "log/slog"
	"os"
	"reflect"
	"strings"
	"sync"

	kernellogging "github.com/rin721/micro-go/internal/kernel/logging"
	capabilitylogging "github.com/rin721/micro-go/types/capability/logging"
)

// ChangeResult 描述基线实现能否原地应用新配置。
type ChangeResult uint8

const (
	// ChangeApplied 表示配置已经安全地原地应用。
	ChangeApplied ChangeResult = iota
	// ChangeRestartRequired 表示 Handler 或输出资源必须通过重建替换。
	ChangeRestartRequired
)

// Config 是 Kernel Slog 基线实现拥有的强类型配置。
type Config struct {
	// Level 是 debug、info、warn 或 error，并支持原地 Reload。
	Level string `yaml:"level" json:"level" validate:"required,oneof=debug info warn error"`
	// Output 是 stdout、stderr 或由实现打开的文件路径。
	Output string `yaml:"output" json:"output" validate:"required"`
	// JSON 选择 JSON Handler；false 使用文本 Handler。
	JSON bool `yaml:"json" json:"json"`
}

// sink 把一套可直接写日志的 Handler、动态级别和自有输出资源绑定为不可拆分基线。
type sink struct {
	// logger 是已经绑定 Handler 的直接项目 Logger 实现。
	logger *directLogger
	// level 支持在不重建 Handler 的情况下原子调整过滤级别。
	level *stdslog.LevelVar
	// config 保存当前已应用配置，用于判断 Reload 是否需要重启。
	config Config
	// closer 仅在 Output 是本实现打开的文件时非 nil。
	closer io.Closer
}

// operation 记录派生 view 上按调用顺序追加的一次 Named 或 With 操作。
type operation struct {
	// name 非空时表示 Named 操作。
	name string
	// fields 非空或 name 为空时表示 With 操作及其字段副本。
	fields []capabilitylogging.Field
}

// Logger 始终保留一个可用的 Slog 基线，并可并发安全地委托给外部 Logger。
// 外部 Logger 的生命周期仍由提供它的 Module 所有。
type Logger struct {
	// mu 保护基线替换、外部委托、配置状态和关闭结果。
	mu sync.RWMutex
	// baseline 始终存在，是配置和外部替换均不可用时的诊断出口。
	baseline *sink
	// replacement 是当前显式选择的增强 Logger，资源仍由其 Module 持有。
	replacement capabilitylogging.Logger
	// configured 防止应用配置被重复作为首次配置写入。
	configured bool
	// closed 阻止关闭后重新配置或替换。
	closed bool
	// closeErr 缓存首次关闭结果，使重复 Close 返回一致错误。
	closeErr error
}

// New 使用指定 Writer 创建 Debug/JSON 基线；nil Writer 使用 stderr。
// 该基线不依赖配置加载，因此注册、配置和构造失败始终存在诊断出口。
func New(writer io.Writer) *Logger {
	// nil Writer 回退 stderr，保证最早启动阶段也具备可见诊断出口。
	if writer == nil {
		writer = os.Stderr
	}
	// 初始级别使用 Debug，避免配置加载前过滤掉诊断信息。
	level := &stdslog.LevelVar{}
	level.Set(stdslog.LevelDebug)
	return &Logger{baseline: newSink(writer, nil, level, Config{Level: "debug", Output: "stderr", JSON: true})}
}

// Configure 首次把应用配置应用到基线；失败时保留原有早期诊断 Writer。
func (l *Logger) Configure(cfg Config) error {
	// 先在锁外解析并打开候选资源，避免持锁执行可能阻塞的文件 I/O。
	levelValue, err := parseLevel(cfg.Level)
	if err != nil {
		return err
	}
	writer, closer, err := output(cfg.Output)
	if err != nil {
		return err
	}
	level := &stdslog.LevelVar{}
	level.Set(levelValue)
	candidate := newSink(writer, closer, level, cfg)

	// 只在候选完整可用后加锁交换，任何失败都保留旧基线。
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		// Logger 已关闭时必须同时关闭刚创建的候选文件，保留两项错误。
		return errors.Join(errors.New("kernel logger is closed"), closeOwned(candidate))
	}
	if l.configured {
		// 首次配置只能发生一次；重复调用同样释放未采用的候选资源。
		return errors.Join(errors.New("kernel logger is already configured"), closeOwned(candidate))
	}
	// 交换后旧的启动 Writer 由调用方拥有，无需关闭；新文件所有权进入 baseline。
	l.baseline = candidate
	l.configured = true
	return nil
}

// Apply 只原地更新 Level；Output 或 JSON 变化要求应用重启。
func (l *Logger) Apply(candidate Config) (ChangeResult, error) {
	// 整个比较和更新过程串行化，确保 config 与 LevelVar 始终代表同一候选。
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return ChangeApplied, errors.New("kernel logger is closed")
	}
	// Apply 只能作用于已接受的应用配置，不能替代 Configure。
	if !l.configured {
		return ChangeApplied, errors.New("kernel logger is not configured")
	}
	// Handler 类型或输出资源变化无法仅更新 LevelVar，交由外部重启重建。
	if candidate.Output != l.baseline.config.Output || candidate.JSON != l.baseline.config.JSON {
		return ChangeRestartRequired, nil
	}
	// 级别必须先解析成功，失败时当前动态级别和 config 均保持原样。
	level, err := parseLevel(candidate.Level)
	if err != nil {
		return ChangeApplied, err
	}
	// LevelVar 更新对现有 Handler 原子可见，随后同步保存已应用配置。
	l.baseline.level.Set(level)
	l.baseline.config = candidate
	return ChangeApplied, nil
}

// Replace 把后续 Kernel 日志委托给外部 Logger，但不接管其关闭责任。
func (l *Logger) Replace(replacement capabilitylogging.Logger) error {
	// 接口非 nil 仍可能包裹 nil 指针，两种情况都不能成为运行期委托目标。
	if replacement == nil || isNilLogger(replacement) {
		return errors.New("kernel logger replacement is nil")
	}
	// 直接替换为自身会让 view.target 递归调用同一个 Manager。
	if value, ok := replacement.(*Logger); ok && value == l {
		return errors.New("kernel logger cannot replace itself")
	}
	// 从当前 Manager 派生的 view 同样会回到 root，必须识别并拒绝间接自替换。
	if value, ok := replacement.(*view); ok && value.root == l {
		return errors.New("kernel logger cannot replace itself through a derived logger")
	}
	// 校验完成后在写锁内检查关闭状态并一次性交换委托。
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return errors.New("kernel logger is closed")
	}
	l.replacement = replacement
	return nil
}

// Restore 恢复 Kernel 自有基线，不关闭此前的外部 Logger。
func (l *Logger) Restore() {
	// Restore 只清除引用，不调用外部 Logger 的 Close。
	l.mu.Lock()
	l.replacement = nil
	l.mu.Unlock()
}

// Close 幂等关闭基线自己打开的文件；不会关闭外部替换 Logger。
func (l *Logger) Close() error {
	// 写锁保证首次关闭、重复关闭和并发日志目标选择之间顺序明确。
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return l.closeErr
	}
	// 先标记关闭并断开外部委托，再只关闭基线自有资源。
	l.closed = true
	l.replacement = nil
	l.closeErr = closeOwned(l.baseline)
	return l.closeErr
}

// Debug 写入 Debug 级别日志。
func (l *Logger) Debug(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	(&view{root: l}).Debug(ctx, message, fields...)
}

// Info 写入 Info 级别日志。
func (l *Logger) Info(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	(&view{root: l}).Info(ctx, message, fields...)
}

// Warn 写入 Warn 级别日志。
func (l *Logger) Warn(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	(&view{root: l}).Warn(ctx, message, fields...)
}

// Error 写入 Error 级别日志。
func (l *Logger) Error(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	(&view{root: l}).Error(ctx, message, fields...)
}

// With 返回动态派生 Logger；替换后仍会把固定字段应用到新的委托目标。
func (l *Logger) With(fields ...capabilitylogging.Field) capabilitylogging.Logger {
	return (&view{root: l}).With(fields...)
}

// Named 返回动态派生 Logger；替换后仍会把命名空间应用到新的委托目标。
func (l *Logger) Named(name string) capabilitylogging.Logger {
	return (&view{root: l}).Named(name)
}

// view 保存相对于根 Manager 的派生操作，实际目标在每次写日志时动态选择。
type view struct {
	// root 是永不变化的 Kernel Logger 管理器。
	root *Logger
	// operations 按 With/Named 调用顺序保存不可变副本。
	operations []operation
}

// Debug 在当前动态目标上重放派生操作后写入 Debug 日志。
func (v *view) Debug(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	v.target().Debug(ctx, message, fields...)
}

// Info 在当前动态目标上重放派生操作后写入 Info 日志。
func (v *view) Info(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	v.target().Info(ctx, message, fields...)
}

// Warn 在当前动态目标上重放派生操作后写入 Warn 日志。
func (v *view) Warn(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	v.target().Warn(ctx, message, fields...)
}

// Error 在当前动态目标上重放派生操作后写入 Error 日志。
func (v *view) Error(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	v.target().Error(ctx, message, fields...)
}

// With 返回追加固定字段操作的新 view，不修改当前 view 的操作切片。
func (v *view) With(fields ...capabilitylogging.Field) capabilitylogging.Logger {
	// 两层复制分别隔离已有操作列表和调用方可变字段切片。
	operations := append([]operation(nil), v.operations...)
	operations = append(operations, operation{fields: append([]capabilitylogging.Field(nil), fields...)})
	return &view{root: v.root, operations: operations}
}

// Named 返回追加命名空间操作的新 view，不提前绑定当前委托目标。
func (v *view) Named(name string) capabilitylogging.Logger {
	// 复制操作列表维持派生 Logger 的值语义。
	operations := append([]operation(nil), v.operations...)
	operations = append(operations, operation{name: name})
	return &view{root: v.root, operations: operations}
}

// target 读取当前委托目标并按原始顺序重放全部派生操作。
func (v *view) target() capabilitylogging.Logger {
	// 读锁只覆盖目标指针读取，后续第三方 Logger 调用不会阻塞 Replace 或 Restore。
	v.root.mu.RLock()
	target := v.root.replacement
	if target == nil {
		// 没有外部替换时始终使用可用的内部基线。
		target = v.root.baseline.logger
	}
	v.root.mu.RUnlock()
	// Named 与 With 的先后会影响结构化输出，因此严格按记录顺序重放。
	for _, current := range v.operations {
		if current.name != "" {
			target = target.Named(current.name)
		} else {
			target = target.With(current.fields...)
		}
	}
	return target
}

// directLogger 把项目 Field 转换为 slog.Attr，并直接写入固定 slog.Logger。
type directLogger struct {
	// logger 是已经配置 Handler、级别和派生属性的标准库 Logger。
	logger *stdslog.Logger
}

// Debug 转换字段并写入标准库 Debug 级别。
func (l *directLogger) Debug(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	l.logger.LogAttrs(ctx, stdslog.LevelDebug, message, attrs(fields)...)
}

// Info 转换字段并写入标准库 Info 级别。
func (l *directLogger) Info(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	l.logger.LogAttrs(ctx, stdslog.LevelInfo, message, attrs(fields)...)
}

// Warn 转换字段并写入标准库 Warn 级别。
func (l *directLogger) Warn(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	l.logger.LogAttrs(ctx, stdslog.LevelWarn, message, attrs(fields)...)
}

// Error 转换字段并写入标准库 Error 级别。
func (l *directLogger) Error(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	l.logger.LogAttrs(ctx, stdslog.LevelError, message, attrs(fields)...)
}

// With 把项目字段转换为 slog 属性并返回共享底层 Handler 的派生 Logger。
func (l *directLogger) With(fields ...capabilitylogging.Field) capabilitylogging.Logger {
	// slog.Logger.With 接受 key/value 或 Attr，因此先构造长度准确的参数切片。
	values := make([]any, 0, len(fields))
	for _, field := range fields {
		values = append(values, stdslog.Any(field.Key, field.Value))
	}
	return &directLogger{logger: l.logger.With(values...)}
}

// Named 使用 slog Group 表达日志命名空间。
func (l *directLogger) Named(name string) capabilitylogging.Logger {
	return &directLogger{logger: l.logger.WithGroup(name)}
}

// newSink 根据 Writer、关闭所有权、动态级别和配置构造一套完整基线。
func newSink(writer io.Writer, closer io.Closer, level *stdslog.LevelVar, cfg Config) *sink {
	// Handler 共享 LevelVar，使 Apply 可以原地改变过滤阈值。
	options := &stdslog.HandlerOptions{Level: level}
	// 文本是默认编码，JSON 配置显式替换 Handler 类型。
	var handler stdslog.Handler = stdslog.NewTextHandler(writer, options)
	if cfg.JSON {
		handler = stdslog.NewJSONHandler(writer, options)
	}
	// directLogger 和资源元数据一同封装，Close 不需要重新推断 Writer 所有权。
	return &sink{logger: &directLogger{logger: stdslog.New(handler)}, level: level, config: cfg, closer: closer}
}

// closeOwned 只关闭 sink 明确拥有的输出资源，并允许 nil 或标准流无操作返回。
func closeOwned(value *sink) error {
	if value == nil || value.closer == nil {
		return nil
	}
	return value.closer.Close()
}

// attrs 把项目字段逐项转换为标准库 Attr，不泄漏 slog 类型到契约层。
func attrs(fields []capabilitylogging.Field) []stdslog.Attr {
	// 预分配准确容量，转换不会保留调用方字段切片。
	result := make([]stdslog.Attr, 0, len(fields))
	for _, field := range fields {
		result = append(result, stdslog.Any(field.Key, field.Value))
	}
	return result
}

// parseLevel 把配置字符串归一化为标准库级别，并拒绝未知值。
func parseLevel(value string) (stdslog.Level, error) {
	// 大小写不影响配置语义，但不裁剪空白以继续暴露配置错误。
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

// output 解析标准流名称或打开由 Logger 负责关闭的私有文件。
func output(value string) (io.Writer, io.Closer, error) {
	// 标准流由进程拥有，因此返回 nil closer；其他值按文件路径处理。
	switch value {
	case "stdout":
		return os.Stdout, nil, nil
	case "stderr":
		return os.Stderr, nil, nil
	default:
		// 追加模式保留既有日志，0600 限制新文件只允许当前用户访问。
		file, err := os.OpenFile(value, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, nil, fmt.Errorf("open kernel slog output: %w", err)
		}
		return file, file, nil
	}
}

// isNilLogger 识别装在接口值中的 nil 引用类型，防止替换后调用 panic。
func isNilLogger(logger capabilitylogging.Logger) bool {
	// ValueOf 在前置非 nil 接口条件下有效；只有可为 nil 的 Kind 需要调用 IsNil。
	value := reflect.ValueOf(logger)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// 编译期断言保证根 Logger 同时满足 Kernel 管理和业务日志契约。
var _ kernellogging.Manager = (*Logger)(nil)

// 编译期断言保证动态派生 view 仍满足业务日志契约。
var _ capabilitylogging.Logger = (*view)(nil)

// 编译期断言保证固定目标 directLogger 满足业务日志契约。
var _ capabilitylogging.Logger = (*directLogger)(nil)
