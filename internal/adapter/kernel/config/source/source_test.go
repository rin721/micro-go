// 本文件从 Source 契约验证文件、环境、Flag、取消和深复制边界。
package source

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/rin721/micro-go/internal/kernel/config"
)

// TestFromFileValidatesPathAndLoadsJSON 同时覆盖创建期路径校验和读取期格式识别。
func TestFromFileValidatesPathAndLoadsJSON(t *testing.T) {
	// 纯空白路径没有所有权语义，必须在文件 I/O 前拒绝。
	if _, err := FromFile("  "); err == nil {
		t.Fatal("FromFile() accepted an empty path")
	}
	// 写入最小 JSON 文件，确保扩展名映射到 FormatJSON 且原始字节被保留。
	path := filepath.Join(t.TempDir(), "app.json")
	if err := os.WriteFile(path, []byte(`{"app":{"name":"demo"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := source.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Source 不解析 JSON，只声明格式并交付非空原始内容。
	if payload.Format != config.FormatJSON || len(payload.Bytes) == 0 {
		t.Fatalf("payload = %+v", payload)
	}
}

// TestFromEnvironmentExcludesProcessControlKeys 验证控制变量不会进入模块配置树。
func TestFromEnvironmentExcludesProcessControlKeys(t *testing.T) {
	// 两个变量共享前缀，但 CONFIG_FILE 被显式排除，LOGGING__LEVEL 应映射为点分键。
	t.Setenv("TESTAPP_CONFIG_FILE", "secret-control-path")
	t.Setenv("TESTAPP_LOGGING__LEVEL", "debug")
	source := FromEnvironment("TESTAPP", "TESTAPP_CONFIG_FILE")
	payload, err := source.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := payload.Values["config_file"]; exists {
		t.Fatalf("excluded control key leaked into payload: %+v", payload.Values)
	}
	if payload.Values["logging.level"] != "debug" {
		t.Fatalf("payload values = %+v", payload.Values)
	}
}

// TestSourcesRespectCancellationAndRejectNilFlags 锁定统一取消检查和 nil FlagSet 边界。
func TestSourcesRespectCancellationAndRejectNilFlags(t *testing.T) {
	// 预先取消 Context，证明即使内存来源无需 I/O 也不会忽略调用方停止信号。
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FromValues(map[string]any{"key": "value"}).Load(ctx); err == nil {
		t.Fatal("FromValues.Load() ignored cancellation")
	}
	// nil FlagSet 不能退回全局 command line，必须明确失败。
	if _, err := FromFlags((*flag.FlagSet)(nil)).Load(context.Background()); err == nil {
		t.Fatal("FromFlags.Load() accepted a nil FlagSet")
	}
}

// TestFromValuesReturnsDeepCopy 验证不同 Load 结果与原始嵌套 map 互不共享可变状态。
func TestFromValuesReturnsDeepCopy(t *testing.T) {
	// 原始值包含嵌套 map，能够暴露仅复制顶层 map 的错误实现。
	original := map[string]any{"nested": map[string]any{"value": "old"}}
	source := FromValues(original)
	first, err := source.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// 主动修改第一次返回值，再次 Load 应重新从隔离输入生成旧值。
	first.Values["nested"].(map[string]any)["value"] = "new"
	second, err := source.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if second.Values["nested"].(map[string]any)["value"] != "old" {
		t.Fatal("source retained caller mutation")
	}
}
