package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rin721/micro-go/adapter/clock/system"
	uuidadapter "github.com/rin721/micro-go/adapter/idgen/uuid"
	slogadapter "github.com/rin721/micro-go/adapter/logging/slog"
	"github.com/rin721/micro-go/capability/clock"
	"github.com/rin721/micro-go/capability/idgen"
	"github.com/rin721/micro-go/capability/logging"
	"github.com/rin721/micro-go/kernel/app"
	"github.com/rin721/micro-go/kernel/config"
	"github.com/rin721/micro-go/kernel/module"
)

type Component struct {
	logger logging.Logger
	clock  clock.Clock
	ids    idgen.Generator
}

func NewComponent(logger logging.Logger, clock clock.Clock, ids idgen.Generator) *Component {
	return &Component{logger: logger.Named("example"), clock: clock, ids: ids}
}
func (c *Component) Run(ctx context.Context) error {
	c.logger.Info(ctx, "micro-go application is running", logging.String("id", c.ids.New()), logging.Time("time", c.clock.Now()))
	return nil
}

type componentModule struct{}

func (componentModule) Name() string { return "example-component" }
func (componentModule) Register(registry module.Registry) error {
	return module.Provide(registry, NewComponent)
}

func main() {
	application, err := app.Build(context.Background(),
		app.WithModules(slogadapter.Module{}, system.Module{}, uuidadapter.Module{}, componentModule{}),
		app.WithConfigSources(config.FromValues(map[string]any{"logging": map[string]any{"level": "info", "output": "stdout", "json": false}})),
	)
	if err == nil {
		err = application.Run(context.Background())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
