// Package koanfadapter 的测试验证多来源覆盖和类型解码，不把 Koanf 行为当作公共契约暴露。
package koanfadapter

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rin721/micro-go/internal/kernel/config"
	configsource "github.com/rin721/micro-go/pkg/adapter/kernel/config/source"
	"github.com/rin721/micro-go/pkg/adapter/kernel/di/compiled"
)

type loaderConfig struct {
	Level string `yaml:"level" validate:"required,oneof=debug info"`
	Port  int    `yaml:"port" validate:"gte=1"`
}

// TestLoaderMergesValuesFileEnvironmentAndFlags 覆盖 Values、YAML 文件、环境变量和 Flag
// 的声明顺序，确保后来源覆盖前来源且最终得到单一强类型 Snapshot。
func TestLoaderMergesValuesFileEnvironmentAndFlags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte("logging:\n  level: info\n  port: 8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_LOGGING__LEVEL", "debug")
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.Int("logging.port", 0, "")
	if err := flags.Parse([]string{"--logging.port=9090"}); err != nil {
		t.Fatal(err)
	}
	typeOf := reflect.TypeOf(loaderConfig{})
	loaded, err := New().Load(context.Background(), 7, []config.Source{
		configsource.FromValues(map[string]any{"logging": map[string]any{"level": "info", "port": 80}}),
		configsource.FromFile(path), configsource.FromEnvironment("APP"), configsource.FromFlags(flags),
	}, []compiled.Config{{Module: "logging", Path: "logging", Type: typeOf}})
	if err != nil {
		t.Fatal(err)
	}
	value := loaded.Values[typeOf].Interface().(loaderConfig)
	if value.Level != "debug" || value.Port != 9090 {
		t.Fatalf("merged config = %+v", value)
	}
}
