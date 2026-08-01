package source

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/rin721/micro-go/internal/kernel/config"
)

func TestFromFileValidatesPathAndLoadsJSON(t *testing.T) {
	if _, err := FromFile("  "); err == nil {
		t.Fatal("FromFile() accepted an empty path")
	}
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
	if payload.Format != config.FormatJSON || len(payload.Bytes) == 0 {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestFromEnvironmentExcludesProcessControlKeys(t *testing.T) {
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

func TestSourcesRespectCancellationAndRejectNilFlags(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FromValues(map[string]any{"key": "value"}).Load(ctx); err == nil {
		t.Fatal("FromValues.Load() ignored cancellation")
	}
	if _, err := FromFlags((*flag.FlagSet)(nil)).Load(context.Background()); err == nil {
		t.Fatal("FromFlags.Load() accepted a nil FlagSet")
	}
}

func TestFromValuesReturnsDeepCopy(t *testing.T) {
	original := map[string]any{"nested": map[string]any{"value": "old"}}
	source := FromValues(original)
	first, err := source.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first.Values["nested"].(map[string]any)["value"] = "new"
	second, err := source.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if second.Values["nested"].(map[string]any)["value"] != "old" {
		t.Fatal("source retained caller mutation")
	}
}
