// Package uuid 使用 Google UUID 实现项目 ID 契约。
package uuid

import (
	googleuuid "github.com/google/uuid"
	"github.com/rin721/micro-go/capability/idgen"
	"github.com/rin721/micro-go/kernel/module"
)

type Generator struct{}

func New() *Generator          { return &Generator{} }
func (*Generator) New() string { return googleuuid.NewString() }

type Module struct{}

func (Module) Name() string { return "idgen-uuid" }
func (Module) Register(registry module.Registry) error {
	if err := module.Provide(registry, New); err != nil {
		return err
	}
	if err := module.Bind[idgen.Generator, *Generator](registry); err != nil {
		return err
	}
	return module.Export[idgen.Generator](registry)
}

var _ idgen.Generator = (*Generator)(nil)
