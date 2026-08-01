//go:build integration && !windows

package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	acceptanceChildEnvironment    = "MICRO_GO_ACCEPTANCE_CHILD"
	acceptanceDatabaseEnvironment = "MICRO_GO_ACCEPTANCE_DATABASE"
	acceptanceLogEnvironment      = "MICRO_GO_ACCEPTANCE_LOG"
	acceptanceAddressEnvironment  = "MICRO_GO_ACCEPTANCE_ADDRESS_FILE"
)

func TestBackendProcessReceivesSIGTERM(t *testing.T) {
	if os.Getenv(acceptanceChildEnvironment) == "1" {
		runAcceptanceChild(t)
		return
	}

	directory := t.TempDir()
	addressFile := filepath.Join(directory, "address")
	commandContext, cancelCommand := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelCommand()
	command := exec.CommandContext(commandContext, os.Args[0], "-test.run=^TestBackendProcessReceivesSIGTERM$")
	command.Env = append(os.Environ(),
		acceptanceChildEnvironment+"=1",
		acceptanceDatabaseEnvironment+"="+filepath.Join(directory, "workitems.db"),
		acceptanceLogEnvironment+"="+filepath.Join(directory, "application.log"),
		acceptanceAddressEnvironment+"="+addressFile,
	)
	var output synchronizedBuffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}

	address := waitForAcceptanceAddress(t, commandContext, addressFile, &output)
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Post("http://"+address+"/v1/work-items", "application/json", strings.NewReader(`{"title":"process signal acceptance"}`))
	if err != nil {
		t.Fatalf("create work item before SIGTERM: %v; child output=%s", err, output.String())
	}
	if closeErr := response.Body.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status=%d; child output=%s", response.StatusCode, output.String())
	}

	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v; child output=%s", err, output.String())
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("child did not exit cleanly after SIGTERM: %v; output=%s", err, output.String())
	}
	if commandContext.Err() != nil {
		t.Fatalf("child exceeded shutdown deadline: %v; output=%s", commandContext.Err(), output.String())
	}
}

func runAcceptanceChild(t *testing.T) {
	t.Helper()
	databasePath := os.Getenv(acceptanceDatabaseEnvironment)
	logPath := os.Getenv(acceptanceLogEnvironment)
	addressFile := os.Getenv(acceptanceAddressEnvironment)
	if databasePath == "" || logPath == "" || addressFile == "" {
		t.Fatal("acceptance child paths must be provided")
	}
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	process := startAcceptanceProcess(t, databasePath, logPath)
	if err := os.WriteFile(addressFile, []byte(process.server.Address()), 0o600); err != nil {
		process.stop(t)
		t.Fatal(err)
	}
	<-ctx.Done()
	process.stop(t)
}

func waitForAcceptanceAddress(t *testing.T, ctx context.Context, addressFile string, output *synchronizedBuffer) string {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		payload, err := os.ReadFile(addressFile)
		if err == nil && strings.TrimSpace(string(payload)) != "" {
			return strings.TrimSpace(string(payload))
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read child address: %v; output=%s", err, output.String())
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for child address: %v; output=%s", ctx.Err(), output.String())
		case <-ticker.C:
		}
	}
}
