// 本文件通过真实默认 Adapter 验证组合根启动、停止、候选拒绝和重启决策。
package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
)

// synchronizedBuffer 允许 Runtime 日志 goroutine 与测试断言并发访问内存输出。
type synchronizedBuffer struct {
	// mu 保护 bytes.Buffer 的所有读写。
	mu sync.Mutex
	// buffer 保存实际诊断文本。
	buffer bytes.Buffer
}

// Write 在线程安全临界区实现 io.Writer。
func (b *synchronizedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

// String 在线程安全临界区返回当前诊断文本。
func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

// TestRunBuildsAndStopsApplication 验证真实组合根能够构造默认技术栈，并在取消后等待资源关闭。
func TestRunBuildsAndStopsApplication(t *testing.T) {
	// 配置把业务日志写入临时文件，测试可据此确认 Running 已真正建立。
	directory := t.TempDir()
	logPath := filepath.Join(directory, "application.log")
	configPath := filepath.Join(directory, "app.yaml")
	content := "logging:\n  level: info\n  output: " + filepath.ToSlash(logPath) + "\n  json: false\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(configFileEnvironment, configPath)

	// Run 在独立 goroutine 阻塞，带缓冲 done 保证结果汇报不会泄漏。
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx) }()

	// 轮询真实启动日志并设置硬截止时间，避免以固定 sleep 制造慢机器假失败。
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, err := os.ReadFile(logPath)
		if err == nil && strings.Contains(string(data), "application started") {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("application did not produce startup log: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// 确认启动后取消根 Context，并等待组合根完成全部关停。
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("application did not stop after context cancellation")
	}
}

// TestRunRejectsMissingConfiguration 确认配置来源失败发生在任何组件开始运行之前。
func TestRunRejectsMissingConfiguration(t *testing.T) {
	// 环境变量选择不存在文件，失败应发生在任何 Module 构造和 Runner 启动前。
	t.Setenv(configFileEnvironment, filepath.Join(t.TempDir(), "missing.yaml"))
	if err := Run(context.Background()); err == nil || !strings.Contains(err.Error(), "read config file") {
		t.Fatalf("Run() error = %v, want missing configuration error", err)
	}
}

// TestRunReportsRejectedReloadThroughKernelLogger 验证非法候选可观察、脱敏且不终止旧配置运行。
func TestRunReportsRejectedReloadThroughKernelLogger(t *testing.T) {
	// 初始合法配置让应用进入 Running，业务与 Kernel 日志写入同一文件。
	directory := t.TempDir()
	logPath := filepath.Join(directory, "application.log")
	configPath := filepath.Join(directory, "app.yaml")
	valid := "logging:\n  level: info\n  output: " + filepath.ToSlash(logPath) + "\n  json: false\n"
	if err := os.WriteFile(configPath, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(configFileEnvironment, configPath)

	diagnostics := &synchronizedBuffer{}
	// diagnosticWriter 捕获配置加载前的早期输出，运行日志随后切换到配置文件。
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, diagnostics) }()

	// 等待真实启动事件后再改写配置，避免与首次文件读取竞争。
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, err := os.ReadFile(logPath)
		if err == nil && strings.Contains(string(data), "application started") {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("application did not start: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// 无效级别故意包含可识别文本，后续同时检查事件出现和敏感候选值未泄漏。
	invalidLevel := "forbidden-level"
	invalid := "logging:\n  level: " + invalidLevel + "\n  output: " + filepath.ToSlash(logPath) + "\n  json: false\n"
	if err := os.WriteFile(configPath, []byte(invalid), 0o600); err != nil {
		cancel()
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	// 候选失败只产生 config.failed；应用应继续运行直到测试主动取消。
	for {
		data, _ := os.ReadFile(logPath)
		output := string(data)
		if strings.Contains(output, "kind=config.failed") {
			if strings.Contains(output, invalidLevel) {
				cancel()
				t.Fatalf("runtime diagnostics leaked configuration value: %s", output)
			}
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("rejected reload was not observable: %s", output)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 被拒绝候选不应成为 Run 错误，取消后必须正常返回。
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run after rejected candidate returned %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("application did not stop")
	}
}

// TestRunReportsAppliedReloadAndRestartRequirement 区分 Level 原地更新与 Output 重建要求。
func TestRunReportsAppliedReloadAndRestartRequirement(t *testing.T) {
	// writeConfig 始终覆盖同一监听文件，参数控制候选 Level 和 Output。
	directory := t.TempDir()
	logPath := filepath.Join(directory, "application.log")
	configPath := filepath.Join(directory, "app.yaml")
	writeConfig := func(level, output string) {
		t.Helper()
		content := "logging:\n  level: " + level + "\n  output: " + filepath.ToSlash(output) + "\n  json: false\n"
		if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// 初始 info 配置启动应用并建立文件监听。
	writeConfig("info", logPath)
	t.Setenv(configFileEnvironment, configPath)

	diagnostics := &synchronizedBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, diagnostics) }()

	// 先观察 Running，确保 Watcher 已建立并完成启动窗口补偿 Reload。
	deadline := time.Now().Add(3 * time.Second)
	for !fileContains(logPath, "state=Running") {
		if time.Now().After(deadline) {
			t.Fatalf("application did not reach Running: %s", readFile(logPath))
		}
		time.Sleep(10 * time.Millisecond)
	}
	// 仅改变 Level 应原地 Applied，并产生新的 config.loaded 事件。
	writeConfig("debug", logPath)
	deadline = time.Now().Add(3 * time.Second)
	for strings.Count(readFile(logPath), "kind=config.loaded") < 1 {
		if time.Now().After(deadline) {
			t.Fatalf("applied reload was not observable: %s", readFile(logPath))
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 改变 Output 需要重建 Handler，Application 应以 ErrRestartRequired 退出。
	writeConfig("debug", filepath.Join(directory, "other.log"))
	select {
	case err := <-done:
		if !errors.Is(err, kernelapp.ErrRestartRequired) {
			t.Fatalf("run error=%v, want ErrRestartRequired", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("application did not request restart: %s", readFile(logPath))
	}
	// 最终 RestartRequired 状态必须在旧 Kernel 日志中可观察。
	if !fileContains(logPath, "state=RestartRequired") {
		t.Fatalf("restart decision was not observable: %s", readFile(logPath))
	}
}

// readFile 返回当前文件文本；轮询场景允许文件暂时不存在，因此忽略读取错误并返回空串。
func readFile(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

// fileContains 判断当前文件快照是否包含目标事件片段。
func fileContains(path, value string) bool { return strings.Contains(readFile(path), value) }
