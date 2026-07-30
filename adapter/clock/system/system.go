// Package system 提供系统时钟适配器。
package system

import (
	"time"

	clockcontract "github.com/rin721/micro-go/capability/clock"
	"github.com/rin721/micro-go/kernel/module"
)

type Clock struct{}

func New() *Clock             { return &Clock{} }
func (*Clock) Now() time.Time { return time.Now() }

type Module struct{}

func (Module) Name() string { return "clock-system" }
func (Module) Register(registry module.Registry) error {
	if err := module.Provide(registry, New); err != nil {
		return err
	}
	if err := module.Bind[clockcontract.Clock, *Clock](registry); err != nil {
		return err
	}
	return module.Export[clockcontract.Clock](registry)
}

var _ clockcontract.Clock = (*Clock)(nil)
