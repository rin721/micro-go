package zap_test

import (
	"context"
	"fmt"
	"os"
	"strings"

	zapadapter "github.com/rin721/micro-go/pkg/adapter/logging/zap"
	"github.com/rin721/micro-go/types/capability/logging"
)

func Example() {
	file, err := os.CreateTemp("", "micro-go-zap-*.log")
	if err != nil {
		panic(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		panic(err)
	}
	defer os.Remove(path)

	config := zapadapter.Config{Level: "info", Output: path}
	logger, err := zapadapter.New(config)
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
	restartConfig.Development = true
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

	fmt.Println(applied == zapadapter.ChangeApplied, restart == zapadapter.ChangeRestartRequired)
	fmt.Println(strings.Contains(string(content), "started"), strings.Contains(string(content), "debug-enabled"))
	// Output:
	// true true
	// true true
}
