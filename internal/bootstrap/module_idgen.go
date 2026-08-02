// 本文件在组合根选择 UUID Adapter，并公开项目 ID Generator Capability。
package bootstrap

import (
	"github.com/rin721/micro-go/internal/kernel/module"
	uuidadapter "github.com/rin721/micro-go/pkg/adapter/idgen/uuid"
	"github.com/rin721/micro-go/types/capability/idgen"
)

// idModule 选择 UUID Generator，并将具体实现作为 ID Generator Capability 导出。
type idModule struct{}

// Name 返回依赖图中的稳定 ID 模块名。
func (idModule) Name() string { return "foundation.id.uuid" }

// Register 声明 UUID 构造、接口绑定和跨模块导出。
func (idModule) Register(registry module.Registry) error {
	// Provider 隔离 google/uuid 具体实现。
	if err := module.Provide(registry, uuidadapter.New); err != nil {
		return err
	}
	// Binding 将项目 Generator 契约解析到同一个具体实例。
	if err := module.Bind[idgen.Generator, *uuidadapter.Generator](registry); err != nil {
		return err
	}
	// 只有项目接口被导出，第三方和 Adapter 类型保持模块私有。
	return module.Export[idgen.Generator](registry)
}
