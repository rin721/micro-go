package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/reload"
)

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
