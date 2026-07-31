package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/reload"
)

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

// TestManagedLoggerReloadTranslatesAdapterResult 验证私有桥接只转换契约，不泄露 Slog 类型。
func TestManagedLoggerReloadTranslatesAdapterResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.log")
	logger, err := newManagedLogger(loggingConfig{Level: "info", Output: path})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := logger.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()

	applied, err := logger.Reload(context.Background(), loggingSnapshot(t, loggingConfig{Level: "debug", Output: path}))
	if err != nil || applied != reload.Applied {
		t.Fatalf("level Reload() = (%v, %v), want Applied", applied, err)
	}
	restart, err := logger.Reload(context.Background(), loggingSnapshot(t, loggingConfig{Level: "debug", Output: filepath.Join(t.TempDir(), "other.log")}))
	if err != nil || restart != reload.RestartRequired {
		t.Fatalf("output Reload() = (%v, %v), want RestartRequired", restart, err)
	}
}

func loggingSnapshot(t *testing.T, value loggingConfig) config.Snapshot {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return config.NewSnapshot(1, time.Now().UTC(), []config.SnapshotEntry{{Type: reflect.TypeOf(value), Data: data, Hash: sha256.Sum256(data)}})
}
