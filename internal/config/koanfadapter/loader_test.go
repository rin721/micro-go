package koanfadapter

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rin721/micro-go/internal/di/compiled"
	"github.com/rin721/micro-go/kernel/config"
)

type loaderConfig struct {
	Level string `yaml:"level" validate:"required,oneof=debug info"`
	Port  int    `yaml:"port" validate:"gte=1"`
}

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
		config.FromValues(map[string]any{"logging": map[string]any{"level": "info", "port": 80}}),
		config.FromFile(path), config.FromEnvironment("APP"), config.FromFlags(flags),
	}, []compiled.Config{{Module: "logging", Path: "logging", Type: typeOf}})
	if err != nil {
		t.Fatal(err)
	}
	value := loaded.Values[typeOf].Interface().(loaderConfig)
	if value.Level != "debug" || value.Port != 9090 {
		t.Fatalf("merged config = %+v", value)
	}
}
