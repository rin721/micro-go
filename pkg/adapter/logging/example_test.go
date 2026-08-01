package logging_test

import (
	"context"
	"fmt"

	"github.com/rin721/micro-go/pkg/adapter/logging/noop"
	"github.com/rin721/micro-go/types/capability/logging"
)

func logReady(logger logging.Logger) {
	logger.Named("process").Info(
		context.Background(),
		"ready",
		logging.String("component", "application.process"),
		logging.Bool("running", true),
	)
}

func Example() {
	logReady(noop.New())

	fmt.Println("consumer only depends on logging.Logger")
	// Output: consumer only depends on logging.Logger
}
