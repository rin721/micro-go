// Package module 把 Work Item 验收系统声明为一个 Kernel Module。
package module

import (
	"github.com/rin721/micro-go/internal/acceptance/adapter/sqliteworkitems"
	"github.com/rin721/micro-go/internal/acceptance/transport/workitemshttp"
	"github.com/rin721/micro-go/internal/acceptance/workitems"
	kernelmodule "github.com/rin721/micro-go/internal/kernel/module"
	"github.com/rin721/micro-go/types/capability/logging"
)

// WorkItems 声明完整验收纵切片。Capture 只让进程级测试获得实际随机监听地址，
// 不进入业务依赖图，也不改变 Server 的资源所有权。
type WorkItems struct {
	Capture func(*workitemshttp.Server)
}

// Name 返回验收 Module 的稳定唯一名称。
func (WorkItems) Name() string { return "acceptance.workitems" }

// Register 声明配置、Store Binding、应用服务和 HTTP Server Provider。
func (m WorkItems) Register(registry kernelmodule.Registry) error {
	if err := kernelmodule.Config[sqliteworkitems.Config](registry, "workitems.database"); err != nil {
		return err
	}
	if err := kernelmodule.Config[workitemshttp.Config](registry, "workitems.http"); err != nil {
		return err
	}
	if err := kernelmodule.Provide(registry, sqliteworkitems.New); err != nil {
		return err
	}
	if err := kernelmodule.Bind[workitems.Repository, *sqliteworkitems.Store](registry); err != nil {
		return err
	}
	if err := kernelmodule.Bind[workitems.Readiness, *sqliteworkitems.Store](registry); err != nil {
		return err
	}
	if err := kernelmodule.Provide(registry, workitems.NewService); err != nil {
		return err
	}
	return kernelmodule.Provide(registry, func(config workitemshttp.Config, service *workitems.Service, readiness workitems.Readiness, logger logging.Logger) *workitemshttp.Server {
		server := workitemshttp.New(config, service, readiness, logger)
		if m.Capture != nil {
			m.Capture(server)
		}
		return server
	})
}
