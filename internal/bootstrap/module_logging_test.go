// 本文件验证组合根日志桥接对配置快照和 Adapter 结果的双向翻译。
package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	kernelslog "github.com/rin721/micro-go/internal/adapter/kernel/logging/slog"
	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/reload"
)

// TestManagedLoggerReloadTranslatesAdapterResult 验证私有桥接只转换契约，不泄露 Slog 类型。
func TestManagedLoggerReloadTranslatesAdapterResult(t *testing.T) {
	// 文件输出让首次 Configure 建立真实自有资源，早期诊断 Writer 丢弃即可。
	path := filepath.Join(t.TempDir(), "application.log")
	kernelLogger := kernelslog.New(io.Discard)
	logger, err := newLoggingModule(kernelLogger).provide(loggingConfig{Level: "info", Output: path})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Kernel Logger 所有权仍在 Bootstrap 测试，managedLogger 不负责 Close。
		if err := kernelLogger.Close(); err != nil {
			t.Error(err)
		}
	}()

	// 相同输出和编码、仅级别变化应翻译为 Kernel Applied。
	applied, err := logger.Reload(context.Background(), loggingSnapshot(t, loggingConfig{Level: "debug", Output: path}))
	if err != nil || applied != reload.Applied {
		t.Fatalf("level Reload() = (%v, %v), want Applied", applied, err)
	}
	// 输出路径变化需要重建 Handler 和文件，桥接为 RestartRequired。
	restart, err := logger.Reload(context.Background(), loggingSnapshot(t, loggingConfig{Level: "debug", Output: filepath.Join(t.TempDir(), "other.log")}))
	if err != nil || restart != reload.RestartRequired {
		t.Fatalf("output Reload() = (%v, %v), want RestartRequired", restart, err)
	}
}

// loggingSnapshot 把测试配置编码为与真实 Loader 相同形状的不可变快照。
func loggingSnapshot(t *testing.T, value loggingConfig) config.Snapshot {
	t.Helper()
	// JSON 同时作为快照 Data 和摘要输入，保持内容一致。
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return config.NewSnapshot(1, time.Now().UTC(), []config.SnapshotEntry{{Type: reflect.TypeOf(value), Data: data, Hash: sha256.Sum256(data)}})
}
