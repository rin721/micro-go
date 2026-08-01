package slog_test

import (
	"context"
	"fmt"
	"os"
	"strings"

	slogadapter "github.com/rin721/micro-go/pkg/adapter/logging/slog"
	"github.com/rin721/micro-go/types/capability/logging"
)

func Example() {
	file, err := os.CreateTemp("", "micro-go-slog-*.log")
	if err != nil {
		panic(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		panic(err)
	}
	defer os.Remove(path)

	config := slogadapter.Config{Level: "info", Output: path, JSON: true}
	logger, err := slogadapter.New(config)
	if err != nil {
		panic(err)
	}
	logger.Named("worker").With(logging.String("component", "example")).Info(context.Background(), "started")

	config.Level = "debug"
	applied, err := logger.Apply(config)
	if err != nil {
		panic(err)
	}
	logger.Debug(context.Background(), "debug-enabled")

	restartConfig := config
	restartConfig.JSON = false
	restart, err := logger.Apply(restartConfig)
	if err != nil {
		panic(err)
	}
	if err := logger.Close(context.Background()); err != nil {
		panic(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	fmt.Println(applied == slogadapter.ChangeApplied, restart == slogadapter.ChangeRestartRequired)
	fmt.Println(strings.Contains(string(content), "started"), strings.Contains(string(content), "debug-enabled"))
	// Output:
	// true true
	// true true
}
