// Package logging_test 从公共契约视角验证不同日志 Adapter 的可替换性。
package logging_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zapadapter "github.com/rin721/micro-go/pkg/adapter/logging/zap"
	"github.com/rin721/micro-go/types/capability/logging"
)

// closeLogger 组合业务 Logger 与测试需要验证的生命周期关闭能力。
type closeLogger interface {
	// Logger 保留全部业务日志方法。
	logging.Logger
	// Close 允许测试验证资源释放和幂等结果。
	Close(context.Context) error
}

// TestZapAdapterHonorsContract 验证公共增强实现保持字段、命名 Logger 和幂等 Close 语义。
func TestZapAdapterHonorsContract(t *testing.T) {
	// 使用临时文件验证真实 Encoder、字段和关闭后的持久内容。
	path := filepath.Join(t.TempDir(), "app.log")
	// 接口变量确保测试只使用约定契约和显式生命周期。
	var logger closeLogger
	logger, err := zapadapter.New(zapadapter.Config{Level: "info", Output: path})
	if err != nil {
		t.Fatal(err)
	}
	logger.Named("contract").With(logging.String("key", "value")).Info(context.Background(), "hello")
	// 连续关闭两次必须返回一致成功，不能重复关闭文件产生错误。
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
	// 日志文本同时包含消息和结构化字段值，证明转换链未丢失信息。
	if !strings.Contains(string(data), "hello") || !strings.Contains(string(data), "value") {
		t.Fatalf("unexpected log: %s", data)
	}
}
