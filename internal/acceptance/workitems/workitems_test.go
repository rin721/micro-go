package workitems

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rin721/micro-go/pkg/adapter/logging/noop"
)

type repositoryStub struct {
	created Item
	item    Item
	err     error
}

func (r *repositoryStub) Create(_ context.Context, item Item) error { r.created = item; return r.err }
func (r *repositoryStub) Get(context.Context, string) (Item, error) { return r.item, r.err }
func (r *repositoryStub) Complete(context.Context, string, time.Time) (Item, error) {
	return r.item, r.err
}

type fixedClock struct{ value time.Time }

func (c fixedClock) Now() time.Time { return c.value }

type fixedID string

func (id fixedID) New() string { return string(id) }

func TestServiceCreatesNormalizedWorkItem(t *testing.T) {
	repository := &repositoryStub{}
	now := time.Date(2026, time.August, 1, 11, 0, 0, 0, time.FixedZone("test", 8*60*60))
	service := NewService(repository, fixedClock{value: now}, fixedID("work-1"), noop.New())
	item, err := service.Create(context.Background(), "  Verify workflow  ")
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != "work-1" || item.Title != "Verify workflow" || item.Status != StatusOpen || !item.CreatedAt.Equal(now.UTC()) || repository.created.ID != item.ID {
		t.Fatalf("created item=%+v repository=%+v", item, repository.created)
	}
}

func TestServiceRejectsInvalidTitlesBeforeRepository(t *testing.T) {
	repository := &repositoryStub{err: errors.New("repository must not be called")}
	service := NewService(repository, fixedClock{}, fixedID("unused"), noop.New())
	for _, title := range []string{"   ", strings.Repeat("界", MaxTitleLength+1)} {
		if _, err := service.Create(context.Background(), title); !errors.Is(err, ErrInvalidTitle) {
			t.Fatalf("Create(%q) error=%v", title, err)
		}
	}
	if repository.created.ID != "" {
		t.Fatalf("repository received invalid item=%+v", repository.created)
	}
}
