// Package logging_test 从公共契约视角验证不同日志 Adapter 的可替换性。
package logging_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	slogadapter "github.com/rin721/micro-go/adapter/logging/slog"
	zapadapter "github.com/rin721/micro-go/adapter/logging/zap"
	"github.com/rin721/micro-go/capability/logging"
)

type closeLogger interface {
	logging.Logger
	Close(context.Context) error
}

// TestLoggingAdaptersHonorContract 用同一组行为断言覆盖 Slog 与 Zap，防止具体实现
// 在字段、命名 Logger 或关闭语义上悄悄偏离 capability 契约。
func TestLoggingAdaptersHonorContract(t *testing.T) {
	tests := []struct {
		name   string
		create func(string) (closeLogger, error)
	}{
		{"zap", func(path string) (closeLogger, error) {
			return zapadapter.New(zapadapter.Config{Level: "info", Output: path})
		}},
		{"slog", func(path string) (closeLogger, error) {
			return slogadapter.New(slogadapter.Config{Level: "info", Output: path, JSON: true})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "app.log")
			logger, err := test.create(path)
			if err != nil {
				t.Fatal(err)
			}
			logger.Named("contract").With(logging.String("key", "value")).Info(context.Background(), "hello")
			if err := logger.Close(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := logger.Close(context.Background()); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "hello") || !strings.Contains(string(data), "value") {
				t.Fatalf("unexpected log: %s", data)
			}
		})
	}
}
