package registration

import (
	"fmt"
	"strings"

	"github.com/rin721/micro-go/kernel/diagnostic"
	"github.com/rin721/micro-go/kernel/module"
)

type Provider struct {
	Module      string
	ModuleOrder int
	Order       int
	Declaration module.ProviderDeclaration
}

type Binding struct {
	Module      string
	ModuleOrder int
	Order       int
	Declaration module.BindingDeclaration
}

type Export struct {
	Module      string
	ModuleOrder int
	Order       int
	Declaration module.ExportDeclaration
}

type Config struct {
	Module      string
	ModuleOrder int
	Order       int
	Declaration module.ConfigDeclaration
}

type Collection struct {
	Providers []Provider
	Bindings  []Binding
	Exports   []Export
	Configs   []Config
}

type Registry struct {
	module      string
	moduleOrder int
	collection  *Collection
	frozen      bool
	order       int
}

func New(moduleName string, moduleOrder int, collection *Collection) *Registry {
	return &Registry{module: moduleName, moduleOrder: moduleOrder, collection: collection}
}

func (r *Registry) Freeze() { r.frozen = true }

func (r *Registry) RegisterProvider(value module.ProviderDeclaration) error {
	if err := r.mutable(); err != nil {
		return err
	}
	r.collection.Providers = append(r.collection.Providers, Provider{r.module, r.moduleOrder, r.next(), value})
	return nil
}

func (r *Registry) RegisterBinding(value module.BindingDeclaration) error {
	if err := r.mutable(); err != nil {
		return err
	}
	r.collection.Bindings = append(r.collection.Bindings, Binding{r.module, r.moduleOrder, r.next(), value})
	return nil
}

func (r *Registry) RegisterExport(value module.ExportDeclaration) error {
	if err := r.mutable(); err != nil {
		return err
	}
	r.collection.Exports = append(r.collection.Exports, Export{r.module, r.moduleOrder, r.next(), value})
	return nil
}

func (r *Registry) RegisterConfig(value module.ConfigDeclaration) error {
	if err := r.mutable(); err != nil {
		return err
	}
	r.collection.Configs = append(r.collection.Configs, Config{r.module, r.moduleOrder, r.next(), value})
	return nil
}

func (r *Registry) mutable() error {
	if r.frozen {
		return fmt.Errorf("registry for module %q is frozen", r.module)
	}
	return nil
}

func (r *Registry) next() int { value := r.order; r.order++; return value }

func Collect(modules []module.Module) (collection Collection, err error) {
	seen := make(map[string]struct{}, len(modules))
	for index, current := range modules {
		if current == nil {
			return collection, fmt.Errorf("module at index %d is nil", index)
		}
		name := strings.TrimSpace(current.Name())
		if name == "" {
			return collection, fmt.Errorf("module at index %d has an empty name", index)
		}
		if _, exists := seen[name]; exists {
			return collection, fmt.Errorf("duplicate module name %q", name)
		}
		seen[name] = struct{}{}
		registry := New(name, index, &collection)
		func() {
			defer func() {
				if value := recover(); value != nil {
					err = fmt.Errorf("register module %q: %w", name, diagnostic.NewPanicError(value))
				}
			}()
			if registerErr := current.Register(registry); registerErr != nil {
				err = fmt.Errorf("register module %q: %w", name, registerErr)
			}
		}()
		registry.Freeze()
		if err != nil {
			return collection, err
		}
	}
	return collection, nil
}
