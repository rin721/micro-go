// Package main 展示 micro-go 的最小完整组合根：选择 Adapter、声明业务组件、提供配置，
// 再由 Application 统一构造和驱动 Runner。
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

// Component 是示例业务组件。它只依赖 capability 接口，不感知 Slog、系统时钟或 UUID 库。
type Component struct {
	logger logging.Logger
	clock  clock.Clock
	ids    idgen.Generator
}

// NewComponent 是普通 Go Provider；参数即依赖，返回值即组件，无需容器标记类型。
func NewComponent(logger logging.Logger, clock clock.Clock, ids idgen.Generator) *Component {
	return &Component{logger: logger.Named("example"), clock: clock, ids: ids}
}

// Run 在所有 Starter 完成后由 Application 监督。示例运行一次即返回，用于演示正常退出链。
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
	// main 是唯一了解具体 Adapter 的组合根；替换日志实现不会修改 Component。
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
