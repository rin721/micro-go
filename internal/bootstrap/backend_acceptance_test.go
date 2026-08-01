//go:build integration

package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acceptancemodule "github.com/rin721/micro-go/internal/acceptance/module"
	"github.com/rin721/micro-go/internal/acceptance/transport/workitemshttp"
	"github.com/rin721/micro-go/internal/acceptance/workitems"
	configwatcher "github.com/rin721/micro-go/internal/adapter/kernel/config/fsnotify"
	koanfadapter "github.com/rin721/micro-go/internal/adapter/kernel/config/koanf"
	configsource "github.com/rin721/micro-go/internal/adapter/kernel/config/source"
	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiler"
	digadapter "github.com/rin721/micro-go/internal/adapter/kernel/di/dig"
	registration "github.com/rin721/micro-go/internal/adapter/kernel/module"
	runtimeadapter "github.com/rin721/micro-go/internal/adapter/kernel/runtime"
	kernelapp "github.com/rin721/micro-go/internal/kernel/app"
)

type acceptanceProcess struct {
	application *runtimeadapter.Application
	server      *workitemshttp.Server
	cancel      context.CancelFunc
	done        chan error
}

func startAcceptanceProcess(t *testing.T, databasePath, logPath string) acceptanceProcess {
	t.Helper()
	var server *workitemshttp.Server
	runtimeValue, err := runtimeadapter.New(runtimeadapter.Dependencies{
		Collector:   registration.NewCollector(),
		Compiler:    compiler.New(),
		Loader:      koanfadapter.New(),
		Constructor: digadapter.New(),
		Watcher:     configwatcher.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	application, err := runtimeValue.Build(context.Background(),
		runtimeadapter.WithModules(
			loggingModule{}, clockModule{}, idModule{},
			acceptancemodule.WorkItems{Capture: func(value *workitemshttp.Server) { server = value }},
		),
		runtimeadapter.WithConfigSources(configsource.FromValues(map[string]any{
			"logging": map[string]any{"level": "info", "output": filepath.ToSlash(logPath), "json": false},
			"workitems": map[string]any{
				"database": map[string]any{"path": filepath.ToSlash(databasePath)},
				"http": map[string]any{
					"listen_address": "127.0.0.1:0",
					"read_timeout":   "2s",
					"write_timeout":  "2s",
					"idle_timeout":   "5s",
					"health_timeout": "1s",
					"max_body_bytes": 4096,
				},
			},
		})),
		runtimeadapter.WithStartupTimeout(5*time.Second),
		runtimeadapter.WithShutdownTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	if server == nil {
		t.Fatal("acceptance module did not capture HTTP server")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for application.State() != kernelapp.Running || server.Address() == "" {
		select {
		case runErr := <-done:
			cancel()
			t.Fatalf("acceptance application exited during startup: %v", runErr)
		default:
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("acceptance application did not become running; state=%s address=%q", application.State(), server.Address())
		}
		time.Sleep(10 * time.Millisecond)
	}
	process := acceptanceProcess{application: application, server: server, cancel: cancel, done: done}
	assertHTTPStatus(t, process, http.MethodGet, "/livez", nil, http.StatusOK, nil)
	assertHTTPStatus(t, process, http.MethodGet, "/readyz", nil, http.StatusOK, nil)
	var status map[string]any
	assertHTTPStatus(t, process, http.MethodGet, "/status", nil, http.StatusOK, &status)
	if status["state"] != "running" || status["started"] != true || status["ready"] != true {
		t.Fatalf("status projection=%v", status)
	}
	return process
}

func (p acceptanceProcess) stop(t *testing.T) {
	t.Helper()
	p.cancel()
	select {
	case err := <-p.done:
		if err != nil {
			t.Fatalf("acceptance application stop error=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("acceptance application did not stop")
	}
	if p.application.State() != kernelapp.Closed {
		t.Fatalf("acceptance application state=%s", p.application.State())
	}
}

func TestBackendWorkItemWorkflowPersistsAcrossRestart(t *testing.T) {
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "workitems.db")
	first := startAcceptanceProcess(t, databasePath, filepath.Join(directory, "first.log"))

	var created workitems.Item
	assertHTTPStatus(t, first, http.MethodPost, "/v1/work-items", strings.NewReader(`{"title":"Verify backend workflow"}`), http.StatusCreated, &created)
	if created.ID == "" || created.Title != "Verify backend workflow" || created.Status != workitems.StatusOpen || created.CreatedAt.IsZero() {
		t.Fatalf("created item=%+v", created)
	}

	var fetched workitems.Item
	assertHTTPStatus(t, first, http.MethodGet, "/v1/work-items/"+created.ID, nil, http.StatusOK, &fetched)
	if fetched.ID != created.ID || fetched.Status != workitems.StatusOpen {
		t.Fatalf("fetched item=%+v", fetched)
	}

	var completed workitems.Item
	assertHTTPStatus(t, first, http.MethodPost, "/v1/work-items/"+created.ID+"/complete", nil, http.StatusOK, &completed)
	if completed.Status != workitems.StatusCompleted || completed.CompletedAt == nil {
		t.Fatalf("completed item=%+v", completed)
	}
	var repeated workitems.Item
	assertHTTPStatus(t, first, http.MethodPost, "/v1/work-items/"+created.ID+"/complete", nil, http.StatusOK, &repeated)
	if repeated.CompletedAt == nil || !repeated.CompletedAt.Equal(*completed.CompletedAt) {
		t.Fatalf("idempotent complete changed completion time: first=%+v repeated=%+v", completed, repeated)
	}
	first.stop(t)

	second := startAcceptanceProcess(t, databasePath, filepath.Join(directory, "second.log"))
	var persisted workitems.Item
	assertHTTPStatus(t, second, http.MethodGet, "/v1/work-items/"+created.ID, nil, http.StatusOK, &persisted)
	if persisted.Status != workitems.StatusCompleted || persisted.CompletedAt == nil {
		t.Fatalf("persisted item=%+v", persisted)
	}
	second.stop(t)
}

func TestBackendHTTPRejectsInvalidInputWithoutLeakingInternals(t *testing.T) {
	directory := t.TempDir()
	process := startAcceptanceProcess(t, filepath.Join(directory, "workitems.db"), filepath.Join(directory, "application.log"))
	defer process.stop(t)

	for _, test := range []struct {
		name string
		path string
		body string
		code string
	}{
		{name: "unknown field", path: "/v1/work-items", body: `{"title":"valid","token":"must-not-echo"}`, code: "invalid_request"},
		{name: "empty title", path: "/v1/work-items", body: `{"title":"   "}`, code: "invalid_title"},
		{name: "title too long", path: "/v1/work-items", body: `{"title":"` + strings.Repeat("x", workitems.MaxTitleLength+1) + `"}`, code: "invalid_title"},
		{name: "body too large", path: "/v1/work-items", body: `{"title":"` + strings.Repeat("x", 5000) + `"}`, code: "invalid_request"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var response map[string]any
			payload := assertHTTPStatus(t, process, http.MethodPost, test.path, strings.NewReader(test.body), http.StatusBadRequest, &response)
			if !bytes.Contains(payload, []byte(test.code)) || bytes.Contains(payload, []byte("must-not-echo")) || bytes.Contains(payload, []byte("workitems.db")) {
				t.Fatalf("unsafe error response=%s", payload)
			}
		})
	}
	assertHTTPStatus(t, process, http.MethodGet, "/v1/work-items/missing", nil, http.StatusNotFound, nil)
}

func assertHTTPStatus(t *testing.T, process acceptanceProcess, method, path string, body io.Reader, wantStatus int, output any) []byte {
	t.Helper()
	request, err := http.NewRequest(method, "http://"+process.server.Address()+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != wantStatus {
		t.Fatalf("%s %s status=%d body=%s want=%d", method, path, response.StatusCode, payload, wantStatus)
	}
	if output != nil {
		if err := json.Unmarshal(payload, output); err != nil {
			t.Fatalf("decode %s %s response %s: %v", method, path, payload, err)
		}
	}
	return payload
}
