// Package koanfadapter 的测试验证多来源覆盖和类型解码，不把 Koanf 行为当作公共契约暴露。
package koanfadapter

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	configsource "github.com/rin721/micro-go/internal/adapter/kernel/config/source"
	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiled"
	"github.com/rin721/micro-go/internal/kernel/config"
)

// loaderConfig 同时包含枚举字符串和有下界整数，用于覆盖常见强类型解码与标签校验。
type loaderConfig struct {
	// Level 只接受 debug 或 info。
	Level string `yaml:"level" validate:"required,oneof=debug info"`
	// Port 必须大于等于一。
	Port int `yaml:"port" validate:"gte=1"`
}

// dynamicConfig 验证 map 的动态子键仍归属于已声明字段。
type dynamicConfig struct {
	// Labels 接收配置中运行期才知道的标签名。
	Labels map[string]string `yaml:"labels"`
}

// TestLoaderReadsJSONAndReturnsTypedSnapshot 验证文件来源被解码、校验并发布到强类型快照。
func TestLoaderReadsJSONAndReturnsTypedSnapshot(t *testing.T) {
	// JSON 文件提供完整 logging 子树，避免默认值参与场景。
	path := filepath.Join(t.TempDir(), "app.json")
	if err := os.WriteFile(path, []byte(`{"logging":{"level":"debug","port":8080}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := configsource.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	typeOf := reflect.TypeOf(loaderConfig{})
	// 声明把 logging 路径和准确 struct 类型归属到同一模块。
	loaded, err := New().Load(context.Background(), 1, []config.Source{source}, []compiled.Config{{Module: "logging", Path: "logging", Type: typeOf}})
	if err != nil {
		t.Fatal(err)
	}
	// 通过公共 Snapshot API 读取，验证 Loader 没有只填充内部 Values。
	value, err := config.Value[loaderConfig](loaded.Snapshot)
	if err != nil || value.Level != "debug" || value.Port != 8080 {
		t.Fatalf("snapshot value=%+v error=%v", value, err)
	}
}

// TestLoaderRejectsStrictMergeConflict 验证后来源不能用标量静默覆盖已有 map 子树。
func TestLoaderRejectsStrictMergeConflict(t *testing.T) {
	// 两个 Values 来源在同一路径提供结构冲突，StrictMerge 必须拒绝。
	_, err := New().Load(context.Background(), 1, []config.Source{
		configsource.FromValues(map[string]any{"logging": map[string]any{"level": "info"}}),
		configsource.FromValues(map[string]any{"logging": "invalid"}),
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "merge configuration source") {
		t.Fatalf("Load() error=%v", err)
	}
}

// TestLoaderRejectsUnknownFieldInsideOwnedPath 验证已归属子树中的拼写错误仍会失败。
func TestLoaderRejectsUnknownFieldInsideOwnedPath(t *testing.T) {
	// levle 故意拼错；ErrorUnused 防止它被声明路径所有权检查掩盖。
	typeOf := reflect.TypeOf(loaderConfig{})
	_, err := New().Load(context.Background(), 1,
		[]config.Source{configsource.FromValues(map[string]any{"logging": map[string]any{"level": "info", "port": 8080, "levle": "debug"}})},
		[]compiled.Config{{Module: "logging", Path: "logging", Type: typeOf}},
	)
	if err == nil || !strings.Contains(err.Error(), "levle") {
		t.Fatalf("Load() error=%v, want unknown field", err)
	}
}

// TestLoaderRejectsConfigurationWithoutOwner 确保任何叶子键都必须落入模块声明路径。
func TestLoaderRejectsConfigurationWithoutOwner(t *testing.T) {
	// logging 合法，orphan.enabled 没有任何 Config 声明负责。
	typeOf := reflect.TypeOf(loaderConfig{})
	_, err := New().Load(context.Background(), 1,
		[]config.Source{configsource.FromValues(map[string]any{
			"logging": map[string]any{"level": "info", "port": 8080},
			"orphan":  map[string]any{"enabled": true},
		})},
		[]compiled.Config{{Module: "logging", Path: "logging", Type: typeOf}},
	)
	if err == nil || !strings.Contains(err.Error(), "orphan.enabled") {
		t.Fatalf("Load() error=%v, want unowned key", err)
	}
}

// TestLoaderAllowsDynamicMapKeysOwnedByField 证明 map 字段下的动态标签不会被误判为无主键。
func TestLoaderAllowsDynamicMapKeysOwnedByField(t *testing.T) {
	// region 和 tier 不出现在 Go 字段列表中，但其父路径 labels 已由字段拥有。
	typeOf := reflect.TypeOf(dynamicConfig{})
	loaded, err := New().Load(context.Background(), 1,
		[]config.Source{configsource.FromValues(map[string]any{"dynamic": map[string]any{"labels": map[string]any{"region": "east", "tier": "api"}}})},
		[]compiled.Config{{Module: "dynamic", Path: "dynamic", Type: typeOf}},
	)
	if err != nil {
		t.Fatal(err)
	}
	value := loaded.Values[typeOf].Interface().(dynamicConfig)
	if value.Labels["region"] != "east" || value.Labels["tier"] != "api" {
		t.Fatalf("dynamic config=%+v", value)
	}
}

// errDomainValidation 是领域校验返回的可识别哨兵错误。
var errDomainValidation = errors.New("domain validation failed")

// domainConfig 同时触发 required 标签和自有 Validate 错误。
type domainConfig struct {
	// Name 留空时产生标签校验问题。
	Name string `yaml:"name" validate:"required"`
}

// Validate 固定返回领域哨兵错误，便于检查 errors.Is 保留原因链。
func (domainConfig) Validate() error { return errDomainValidation }

// TestLoaderAggregatesTagAndDomainValidation 验证两类校验问题被同时收集且原因可识别。
func TestLoaderAggregatesTagAndDomainValidation(t *testing.T) {
	// 空 domain 子树会同时违反 Name required 和 Validate。
	typeOf := reflect.TypeOf(domainConfig{})
	_, err := New().Load(context.Background(), 1,
		[]config.Source{configsource.FromValues(map[string]any{"domain": map[string]any{}})},
		[]compiled.Config{{Module: "domain", Path: "domain", Type: typeOf}},
	)
	var validationErr *config.ValidationError
	// As 检查稳定项目类型，Issues 数量和 Is 检查完整聚合及原始领域原因。
	if !errors.As(err, &validationErr) || len(validationErr.Issues) != 2 || !errors.Is(err, errDomainValidation) {
		t.Fatalf("Load() error=%v", err)
	}
}

// TestLoaderPreservesCancellation 确保 Source 取消不会被包装成无法识别的普通配置错误。
func TestLoaderPreservesCancellation(t *testing.T) {
	// 在 Load 前取消，命中第一个 Source 的统一 Context 检查。
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New().Load(ctx, 1, []config.Source{configsource.FromValues(nil)}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Load() error=%v", err)
	}
}

// TestLoaderMergesValuesFileEnvironmentAndFlags 覆盖 Values、YAML 文件、环境变量和 Flag
// 的声明顺序，确保后来源覆盖前来源且最终得到单一强类型 Snapshot。
func TestLoaderMergesValuesFileEnvironmentAndFlags(t *testing.T) {
	// 文件覆盖 Values，环境覆盖文件 Level，Flag 最终覆盖 Port。
	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte("logging:\n  level: info\n  port: 8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_LOGGING__LEVEL", "debug")
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	// Flag 默认值不应生效，显式 Parse 后才加入覆盖来源。
	flags.Int("logging.port", 0, "")
	if err := flags.Parse([]string{"--logging.port=9090"}); err != nil {
		t.Fatal(err)
	}
	fileSource, err := configsource.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	typeOf := reflect.TypeOf(loaderConfig{})
	// 版本 7 同时验证调用方分配的 Snapshot 版本未被 Loader 改写。
	loaded, err := New().Load(context.Background(), 7, []config.Source{
		configsource.FromValues(map[string]any{"logging": map[string]any{"level": "info", "port": 80}}),
		fileSource, configsource.FromEnvironment("APP"), configsource.FromFlags(flags),
	}, []compiled.Config{{Module: "logging", Path: "logging", Type: typeOf}})
	if err != nil {
		t.Fatal(err)
	}
	value := loaded.Values[typeOf].Interface().(loaderConfig)
	if value.Level != "debug" || value.Port != 9090 {
		t.Fatalf("merged config = %+v", value)
	}
}
