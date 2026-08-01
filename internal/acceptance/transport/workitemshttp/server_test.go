package workitemshttp

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rin721/micro-go/internal/acceptance/workitems"
	"github.com/rin721/micro-go/pkg/adapter/clock/system"
	uuidadapter "github.com/rin721/micro-go/pkg/adapter/idgen/uuid"
	"github.com/rin721/micro-go/pkg/adapter/logging/noop"
)

type blockingRepository struct {
	started chan struct{}
	release chan struct{}
}

func (r *blockingRepository) Create(ctx context.Context, _ workitems.Item) error {
	close(r.started)
	select {
	case <-r.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (*blockingRepository) Get(context.Context, string) (workitems.Item, error) {
	return workitems.Item{}, workitems.ErrNotFound
}

func (*blockingRepository) Complete(context.Context, string, time.Time) (workitems.Item, error) {
	return workitems.Item{}, workitems.ErrNotFound
}

type readyDependency struct{}

func (readyDependency) Ready(context.Context) error { return nil }

func TestStopWaitsForInFlightRequest(t *testing.T) {
	repository := &blockingRepository{started: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() {
		select {
		case <-repository.release:
		default:
			close(repository.release)
		}
	})
	service := workitems.NewService(repository, system.New(), uuidadapter.New(), noop.New())
	server := New(Config{
		ListenAddress: "127.0.0.1:0",
		ReadTimeout:   time.Second,
		WriteTimeout:  time.Second,
		IdleTimeout:   time.Second,
		HealthTimeout: time.Second,
		MaxBodyBytes:  4096,
	}, service, readyDependency{}, noop.New())
	if err := server.Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if err := server.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	runCtx, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- server.Run(runCtx) }()

	requestDone := make(chan int, 1)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		response, err := client.Post("http://"+server.Address()+"/v1/work-items", "application/json", strings.NewReader(`{"title":"drain me"}`))
		if err != nil {
			requestDone <- 0
			return
		}
		defer response.Body.Close()
		requestDone <- response.StatusCode
	}()
	select {
	case <-repository.started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not reach repository")
	}

	cancelRun()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelShutdown()
	stopDone := make(chan error, 1)
	go func() { stopDone <- server.Stop(shutdownCtx) }()
	select {
	case err := <-stopDone:
		t.Fatalf("Stop returned before in-flight request completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(repository.release)
	if status := <-requestDone; status != http.StatusCreated {
		t.Fatalf("in-flight request status=%d", status)
	}
	if err := <-stopDone; err != nil {
		t.Fatal(err)
	}
	if err := <-runDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error=%v, want context.Canceled", err)
	}
}
