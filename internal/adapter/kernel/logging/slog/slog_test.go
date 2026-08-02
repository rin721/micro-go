// 本文件验证 Kernel Slog 的配置事务、动态派生、替换所有权和并发安全。
package slog

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	capabilitylogging "github.com/rin721/micro-go/types/capability/logging"
)

// TestLoggerConfigureApplyAndClose 覆盖首次文件配置、级别更新、重启判定和幂等关闭。
func TestLoggerConfigureApplyAndClose(t *testing.T) {
	// nil 早期 Writer 会回退 stderr，Configure 随后切换到测试文件。
	path := filepath.Join(t.TempDir(), "kernel.log")
	logger := New(nil)
	if err := logger.Configure(Config{Level: "info", Output: path, JSON: true}); err != nil {
		t.Fatal(err)
	}
	logger.Debug(context.Background(), "hidden")
	// info 在初始阈值可见，并携带项目结构化字段。
	logger.Info(context.Background(), "visible", capabilitylogging.String("key", "value"))
	// 只改变 Level 应原地应用并让后续 Debug 可见。
	result, err := logger.Apply(Config{Level: "debug", Output: path, JSON: true})
	if err != nil || result != ChangeApplied {
		t.Fatalf("Apply() = (%v, %v), want ChangeApplied", result, err)
	}
	logger.Debug(context.Background(), "now-visible")
	// Output 变化需要重建文件资源，只返回重启要求而不创建 other.log。
	restart, err := logger.Apply(Config{Level: "debug", Output: filepath.Join(t.TempDir(), "other.log"), JSON: true})
	if err != nil || restart != ChangeRestartRequired {
		t.Fatalf("output Apply() = (%v, %v), want ChangeRestartRequired", restart, err)
	}
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	// 第二次 Close 返回缓存结果，不重复关闭文件。
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	output := string(data)
	// hidden 被过滤，其余消息和字段都必须持久化。
	if strings.Contains(output, "hidden") || !strings.Contains(output, "visible") || !strings.Contains(output, "now-visible") || !strings.Contains(output, "value") {
		t.Fatalf("unexpected output: %s", output)
	}
}

// TestLoggerSupportsTextAndJSON 表驱动验证两种 Handler 编码和派生上下文。
func TestLoggerSupportsTextAndJSON(t *testing.T) {
	tests := []struct {
		// name 标识编码子场景。
		name string
		// json 选择 JSON 或文本 Handler。
		json bool
		// want 是对应编码的稳定级别片段。
		want string
	}{
		{name: "text", json: false, want: "level=INFO"},
		{name: "json", json: true, want: `"level":"INFO"`},
	}
	// 每个子场景使用独立文件和 Logger，防止 Handler 配置交叉污染。
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "kernel.log")
			logger := New(nil)
			if err := logger.Configure(Config{Level: "info", Output: path, JSON: test.json}); err != nil {
				t.Fatal(err)
			}
			logger.Named("kernel").With(capabilitylogging.String("key", "value")).Info(context.Background(), "ready")
			// Close 确保文件句柄释放后再读取内容。
			if err := logger.Close(); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if logged := string(content); !strings.Contains(logged, test.want) || !strings.Contains(logged, "ready") || !strings.Contains(logged, "value") {
				t.Fatalf("unexpected %s output: %s", test.name, logged)
			}
		})
	}
}

// TestLoggerConfigureFailureKeepsEarlyBaseline 验证非法候选不会替换启动期诊断出口。
func TestLoggerConfigureFailureKeepsEarlyBaseline(t *testing.T) {
	// bytes.Buffer 精确观察 Configure 失败后仍由旧基线接收日志。
	var output bytes.Buffer
	logger := New(&output)
	if err := logger.Configure(Config{Level: "invalid", Output: "stdout"}); err == nil {
		t.Fatal("Configure() accepted invalid level")
	}
	logger.Error(context.Background(), "still-available")
	// 消息存在证明失败候选没有清空或关闭当前基线。
	if !strings.Contains(output.String(), "still-available") {
		t.Fatalf("early baseline was lost after configuration failure: %s", output.String())
	}
}

// TestLoggerDerivedViewFollowsReplaceAndRestore 验证替换前创建的 view 动态跟随当前目标。
func TestLoggerDerivedViewFollowsReplaceAndRestore(t *testing.T) {
	// derived 在 Replace 之前创建，内部只记录 root 和操作序列。
	var baseline bytes.Buffer
	logger := New(&baseline)
	derived := logger.Named("kernel").With(capabilitylogging.String("fixed", "field"))
	replacement := &recordingLogger{}
	if err := logger.Replace(replacement); err != nil {
		t.Fatal(err)
	}
	derived.Info(context.Background(), "replacement")
	// 替换目标必须收到 Named、With 和本次消息的完整重放结果。
	if !strings.Contains(replacement.String(), "kernel") || !strings.Contains(replacement.String(), "fixed=field") || !strings.Contains(replacement.String(), "replacement") {
		t.Fatalf("replacement did not receive derived context: %s", replacement.String())
	}
	logger.Restore()
	// 同一个 derived 随后应自动回到基线，而无需重新派生。
	derived.Info(context.Background(), "baseline")
	if !strings.Contains(baseline.String(), "baseline") || !strings.Contains(baseline.String(), "field") {
		t.Fatalf("baseline did not receive restored view: %s", baseline.String())
	}
}

// TestLoggerRejectsNilAndSelfReplacement 覆盖接口 typed nil、直接自身和派生自身三种递归目标。
func TestLoggerRejectsNilAndSelfReplacement(t *testing.T) {
	logger := New(nil)
	var typedNil *recordingLogger
	if err := logger.Replace(typedNil); err == nil {
		t.Fatal("Replace() accepted typed nil")
	}
	if err := logger.Replace(logger); err == nil {
		t.Fatal("Replace() accepted itself")
	}
	if err := logger.Replace(logger.Named("derived")); err == nil {
		t.Fatal("Replace() accepted its derived logger")
	}
}

// TestLoggerDoesNotCloseReplacement 确保 Manager.Close 只释放基线自有资源。
func TestLoggerDoesNotCloseReplacement(t *testing.T) {
	logger := New(nil)
	replacement := &recordingLogger{}
	if err := logger.Replace(replacement); err != nil {
		t.Fatal(err)
	}
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	// replacement 的所有权属于外部 Module，closed 必须保持 false。
	if replacement.closed {
		t.Fatal("Close() closed externally owned replacement")
	}
}

// TestLoggerSupportsConcurrentWriteReplaceAndRestore 为 race 门禁制造高频读写目标切换。
func TestLoggerSupportsConcurrentWriteReplaceAndRestore(t *testing.T) {
	// lockedBuffer 让基线 Writer 本身也可安全接收并发写入。
	logger := New(&lockedBuffer{})
	replacement := &recordingLogger{}
	ctx := context.Background()
	var writers sync.WaitGroup
	// 八个 writer 各自复用派生 view，持续读取动态目标。
	for worker := 0; worker < 8; worker++ {
		writers.Add(1)
		go func(worker int) {
			defer writers.Done()
			derived := logger.Named("worker").With(capabilitylogging.Int("id", worker))
			for index := 0; index < 200; index++ {
				derived.Info(ctx, "message", capabilitylogging.Int("index", index))
			}
		}(worker)
	}
	// 主 goroutine 同时反复 Replace/Restore，race 检测器验证锁边界。
	for index := 0; index < 200; index++ {
		if err := logger.Replace(replacement); err != nil {
			t.Fatal(err)
		}
		logger.Restore()
	}
	writers.Wait()
}

// lockedBuffer 为并发日志测试提供线程安全的内存 Writer。
type lockedBuffer struct {
	// mu 保护底层 bytes.Buffer。
	mu sync.Mutex
	// buffer 保存实际写入内容。
	buffer bytes.Buffer
}

// Write 在锁内实现 io.Writer。
func (b *lockedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(value)
}

// String 在锁内返回当前内容副本。
func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

// recordingLogger 记录收到的级别、消息、字段和派生前缀，供目标切换断言。
type recordingLogger struct {
	// mu 保护 parts 和 closed。
	mu sync.Mutex
	// parts 按调用顺序保存可读片段。
	parts []string
	// closed 用于检测 Manager 是否越权关闭替换目标。
	closed bool
}

// Debug 记录 Debug 调用。
func (l *recordingLogger) Debug(_ context.Context, message string, fields ...capabilitylogging.Field) {
	l.record("debug", message, fields)
}

// Info 记录 Info 调用。
func (l *recordingLogger) Info(_ context.Context, message string, fields ...capabilitylogging.Field) {
	l.record("info", message, fields)
}

// Warn 记录 Warn 调用。
func (l *recordingLogger) Warn(_ context.Context, message string, fields ...capabilitylogging.Field) {
	l.record("warn", message, fields)
}

// Error 记录 Error 调用。
func (l *recordingLogger) Error(_ context.Context, message string, fields ...capabilitylogging.Field) {
	l.record("error", message, fields)
}

// With 返回把固定字段编码到 prefix 的转发 Logger。
func (l *recordingLogger) With(fields ...capabilitylogging.Field) capabilitylogging.Logger {
	copy := &recordingLogger{parts: append([]string(nil), l.parts...)}
	copy.record("with", "", fields)
	return &forwardingLogger{target: l, prefix: copy.String()}
}

// Named 返回把名称作为 prefix 的转发 Logger。
func (l *recordingLogger) Named(name string) capabilitylogging.Logger {
	return &forwardingLogger{target: l, prefix: name}
}

// record 在线程安全临界区追加一次完整日志调用。
func (l *recordingLogger) record(level, message string, fields []capabilitylogging.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.parts = append(l.parts, level, message)
	// 字段转换为 key=value，便于字符串包含断言。
	for _, field := range fields {
		l.parts = append(l.parts, field.Key+"="+fieldValue(field.Value))
	}
}

// String 返回全部记录片段的空格连接快照。
func (l *recordingLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.parts, " ")
}

// forwardingLogger 把派生前缀添加到消息后转发给共享 recordingLogger。
type forwardingLogger struct {
	// target 是所有派生实例共享的最终记录器。
	target *recordingLogger
	// prefix 保存按顺序累积的名称和固定字段。
	prefix string
}

// Debug 转发带前缀的 Debug 日志。
func (l *forwardingLogger) Debug(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	l.target.Debug(ctx, l.prefix+" "+message, fields...)
}

// Info 转发带前缀的 Info 日志。
func (l *forwardingLogger) Info(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	l.target.Info(ctx, l.prefix+" "+message, fields...)
}

// Warn 转发带前缀的 Warn 日志。
func (l *forwardingLogger) Warn(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	l.target.Warn(ctx, l.prefix+" "+message, fields...)
}

// Error 转发带前缀的 Error 日志。
func (l *forwardingLogger) Error(ctx context.Context, message string, fields ...capabilitylogging.Field) {
	l.target.Error(ctx, l.prefix+" "+message, fields...)
}

// With 返回继续累积字段前缀的新转发实例。
func (l *forwardingLogger) With(fields ...capabilitylogging.Field) capabilitylogging.Logger {
	parts := []string{l.prefix}
	for _, field := range fields {
		parts = append(parts, field.Key+"="+fieldValue(field.Value))
	}
	return &forwardingLogger{target: l.target, prefix: strings.Join(parts, " ")}
}

// Named 返回继续累积名称前缀的新转发实例。
func (l *forwardingLogger) Named(name string) capabilitylogging.Logger {
	return &forwardingLogger{target: l.target, prefix: strings.TrimSpace(l.prefix + " " + name)}
}

// fieldValue 为测试记录器提供稳定的简化字段文本。
func fieldValue(value any) string {
	// 字符串保留真实值，其他类型只需证明字段存在而不关心格式化细节。
	if text, ok := value.(string); ok {
		return text
	}
	return "value"
}
