package noop_test

import (
	"context"
	"fmt"

	"github.com/rin721/micro-go/pkg/adapter/logging/noop"
	"github.com/rin721/micro-go/types/capability/logging"
)

func Example() {
	var logger logging.Logger = noop.New()
	derived := logger.With(logging.String("component", "test")).Named("worker")
	derived.Info(context.Background(), "discarded")

	fmt.Println(derived == logger)
	// Output: true
}
