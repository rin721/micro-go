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

type loaderConfig struct {
	Level string `yaml:"level" validate:"required,oneof=debug info"`
	Port  int    `yaml:"port" validate:"gte=1"`
}

func TestLoaderReadsJSONAndReturnsTypedSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	if err := os.WriteFile(path, []byte(`{"logging":{"level":"debug","port":8080}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := configsource.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	typeOf := reflect.TypeOf(loaderConfig{})
	loaded, err := New().Load(context.Background(), 1, []config.Source{source}, []compiled.Config{{Module: "logging", Path: "logging", Type: typeOf}})
	if err != nil {
		t.Fatal(err)
	}
	value, err := config.Value[loaderConfig](loaded.Snapshot)
	if err != nil || value.Level != "debug" || value.Port != 8080 {
		t.Fatalf("snapshot value=%+v error=%v", value, err)
	}
}

func TestLoaderRejectsStrictMergeConflict(t *testing.T) {
	_, err := New().Load(context.Background(), 1, []config.Source{
		configsource.FromValues(map[string]any{"logging": map[string]any{"level": "info"}}),
		configsource.FromValues(map[string]any{"logging": "invalid"}),
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "merge configuration source") {
		t.Fatalf("Load() error=%v", err)
	}
}

var errDomainValidation = errors.New("domain validation failed")

type domainConfig struct {
	Name string `yaml:"name" validate:"required"`
}

func (domainConfig) Validate() error { return errDomainValidation }

func TestLoaderAggregatesTagAndDomainValidation(t *testing.T) {
	typeOf := reflect.TypeOf(domainConfig{})
	_, err := New().Load(context.Background(), 1,
		[]config.Source{configsource.FromValues(map[string]any{"domain": map[string]any{}})},
		[]compiled.Config{{Module: "domain", Path: "domain", Type: typeOf}},
	)
	var validationErr *config.ValidationError
	if !errors.As(err, &validationErr) || len(validationErr.Issues) != 2 || !errors.Is(err, errDomainValidation) {
		t.Fatalf("Load() error=%v", err)
	}
}

func TestLoaderPreservesCancellation(t *testing.T) {
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
	fileSource, err := configsource.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	typeOf := reflect.TypeOf(loaderConfig{})
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
