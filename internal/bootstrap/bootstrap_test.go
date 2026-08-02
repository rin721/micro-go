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

type synchronizedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *synchronizedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

// TestRunBuildsAndStopsApplication 验证真实组合根能够构造默认技术栈，并在取消后等待资源关闭。
func TestRunBuildsAndStopsApplication(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "application.log")
	configPath := filepath.Join(directory, "app.yaml")
	content := "logging:\n  level: info\n  output: " + filepath.ToSlash(logPath) + "\n  json: false\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(configFileEnvironment, configPath)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx) }()

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
	t.Setenv(configFileEnvironment, filepath.Join(t.TempDir(), "missing.yaml"))
	if err := Run(context.Background()); err == nil || !strings.Contains(err.Error(), "read config file") {
		t.Fatalf("Run() error = %v, want missing configuration error", err)
	}
}

func TestRunReportsRejectedReloadThroughDefaultObserver(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "application.log")
	configPath := filepath.Join(directory, "app.yaml")
	valid := "logging:\n  level: info\n  output: " + filepath.ToSlash(logPath) + "\n  json: false\n"
	if err := os.WriteFile(configPath, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(configFileEnvironment, configPath)

	diagnostics := &synchronizedBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, diagnostics) }()

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
	invalidLevel := "forbidden-level"
	invalid := "logging:\n  level: " + invalidLevel + "\n  output: " + filepath.ToSlash(logPath) + "\n  json: false\n"
	if err := os.WriteFile(configPath, []byte(invalid), 0o600); err != nil {
		cancel()
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		output := diagnostics.String()
		if strings.Contains(output, `"kind":"config.failed"`) {
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

func TestRunReportsAppliedReloadAndRestartRequirement(t *testing.T) {
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
	writeConfig("info", logPath)
	t.Setenv(configFileEnvironment, configPath)

	diagnostics := &synchronizedBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, diagnostics) }()

	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(diagnostics.String(), `"state":"Running"`) {
		if time.Now().After(deadline) {
			t.Fatalf("application did not reach Running: %s", diagnostics.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	writeConfig("debug", logPath)
	deadline = time.Now().Add(3 * time.Second)
	for strings.Count(diagnostics.String(), `"kind":"config.loaded"`) < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("applied reload was not observable: %s", diagnostics.String())
		}
		time.Sleep(10 * time.Millisecond)
	}

	writeConfig("debug", filepath.Join(directory, "other.log"))
	select {
	case err := <-done:
		if !errors.Is(err, kernelapp.ErrRestartRequired) {
			t.Fatalf("run error=%v, want ErrRestartRequired", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("application did not request restart: %s", diagnostics.String())
	}
	if !strings.Contains(diagnostics.String(), `"state":"RestartRequired"`) {
		t.Fatalf("restart decision was not observable: %s", diagnostics.String())
	}
}
