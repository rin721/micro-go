package bootstrap

import (
	"github.com/rin721/micro-go/internal/kernel/module"
	uuidadapter "github.com/rin721/micro-go/pkg/adapter/idgen/uuid"
	"github.com/rin721/micro-go/types/capability/idgen"
)

// idModule 选择 UUID Generator，并将具体实现作为 ID Generator Capability 导出。
type idModule struct{}

func (idModule) Name() string { return "foundation.id.uuid" }

func (idModule) Register(registry module.Registry) error {
	if err := module.Provide(registry, uuidadapter.New); err != nil {
		return err
	}
	if err := module.Bind[idgen.Generator, *uuidadapter.Generator](registry); err != nil {
		return err
	}
	return module.Export[idgen.Generator](registry)
}
