package sqliteworkitems

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rin721/micro-go/internal/acceptance/workitems"
)

func TestStoreMigratesPersistsAndBecomesUnreadyAfterClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workitems.db")
	store := New(Config{Path: path})
	if err := store.Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	var migrationCount int
	if err := store.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, migrationVersion).Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if migrationCount != 1 {
		t.Fatalf("migration count=%d", migrationCount)
	}
	createdAt := time.Date(2026, time.August, 1, 10, 0, 0, 0, time.UTC)
	item := workitems.Item{ID: "work-1", Title: "Validate persistence", Status: workitems.StatusOpen, CreatedAt: createdAt}
	if err := store.Create(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(context.Background()); err == nil {
		t.Fatal("closed store remained ready")
	}

	reopened := New(Config{Path: path})
	if err := reopened.Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	fetched, err := reopened.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fetched.ID != item.ID || fetched.Title != item.Title || !fetched.CreatedAt.Equal(createdAt) {
		t.Fatalf("fetched item=%+v", fetched)
	}
}

func TestStoreReturnsStableNotFoundError(t *testing.T) {
	store := New(Config{Path: filepath.Join(t.TempDir(), "workitems.db")})
	if err := store.Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if _, err := store.Get(context.Background(), "missing"); !errors.Is(err, workitems.ErrNotFound) {
		t.Fatalf("Get() error=%v", err)
	}
	if _, err := store.Complete(context.Background(), "missing", time.Now()); !errors.Is(err, workitems.ErrNotFound) {
		t.Fatalf("Complete() error=%v", err)
	}
}
