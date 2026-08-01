package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	configsource "github.com/rin721/micro-go/internal/adapter/kernel/config/source"
	"github.com/rin721/micro-go/internal/kernel/config"
)

func TestWatchRejectsMissingDirectory(t *testing.T) {
	source, err := configsource.FromFile(filepath.Join(t.TempDir(), "missing", "app.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	events, failures, err := Watch(context.Background(), []config.Source{source})
	if err == nil || events != nil || failures != nil {
		t.Fatalf("Watch() = (%v, %v, %v)", events, failures, err)
	}
}

func TestWatchClosesChannelsAfterCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte("app: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := configsource.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	events, failures, err := Watch(ctx, []config.Source{source})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	deadline := time.After(time.Second)
	for events != nil || failures != nil {
		select {
		case _, ok := <-events:
			if !ok {
				events = nil
			}
		case _, ok := <-failures:
			if !ok {
				failures = nil
			}
		case <-deadline:
			t.Fatal("watch channels did not close after cancellation")
		}
	}
}

func TestWatchReportsAtomicFileReplacement(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "app.yaml")
	if err := os.WriteFile(path, []byte("app: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := configsource.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, failures, err := Watch(ctx, []config.Source{source})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, filepath.Join(directory, "app.previous.yaml")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
	case err := <-failures:
		t.Fatalf("watch failure: %v", err)
	case <-time.After(time.Second):
		t.Fatal("atomic replacement did not produce a file event")
	}
}
