package compiled

import (
	"reflect"

	"github.com/rin721/micro-go/kernel/di"
)

type Dependency struct {
	Requested reflect.Type
	Resolved  reflect.Type
	Config    bool
}

type Provider struct {
	ID           string
	Module       string
	ModuleOrder  int
	Order        int
	Name         string
	Constructor  reflect.Value
	Type         reflect.Type
	Dependencies []Dependency
}

type Binding struct {
	Module         string
	Contract       reflect.Type
	Implementation reflect.Type
}

type Config struct {
	Module string
	Path   string
	Type   reflect.Type
}

type Plan struct {
	Providers []Provider
	Bindings  []Binding
	Configs   []Config
	Graph     di.Graph
}

type Instance struct {
	Provider Provider
	Value    any
}
