package compiler

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"github.com/rin721/micro-go/internal/di/compiled"
	"github.com/rin721/micro-go/internal/registration"
	"github.com/rin721/micro-go/kernel/di"
	"github.com/rin721/micro-go/kernel/lifecycle"
	"github.com/rin721/micro-go/kernel/module"
)

var (
	errorType    = reflect.TypeOf((*error)(nil)).Elem()
	contextType  = reflect.TypeOf((*context.Context)(nil)).Elem()
	registryType = reflect.TypeOf((*module.Registry)(nil)).Elem()
)

type providerMeta struct {
	value compiled.Provider
	index int
}

func Compile(collection registration.Collection) (*compiled.Plan, error) {
	providers := make(map[reflect.Type]*providerMeta, len(collection.Providers))
	ordered := make([]*providerMeta, 0, len(collection.Providers))
	for index, declaration := range collection.Providers {
		provider, err := inspectProvider(declaration)
		if err != nil {
			return nil, err
		}
		if previous, exists := providers[provider.Type]; exists {
			return nil, fmt.Errorf("duplicate provider result %s in modules %q and %q", provider.Type, previous.value.Module, provider.Module)
		}
		meta := &providerMeta{value: provider, index: index}
		providers[provider.Type] = meta
		ordered = append(ordered, meta)
	}

	configs := make(map[reflect.Type]compiled.Config, len(collection.Configs))
	for _, declaration := range collection.Configs {
		typeOf := declaration.Declaration.Type
		if typeOf == nil || typeOf.Kind() != reflect.Struct {
			return nil, fmt.Errorf("module %q configuration must be a struct value, got %v", declaration.Module, typeOf)
		}
		if strings.TrimSpace(declaration.Declaration.Path) == "" {
			return nil, fmt.Errorf("module %q configuration %s has an empty path", declaration.Module, typeOf)
		}
		if previous, exists := configs[typeOf]; exists {
			return nil, fmt.Errorf("configuration %s declared by both %q and %q", typeOf, previous.Module, declaration.Module)
		}
		if provider, exists := providers[typeOf]; exists {
			return nil, fmt.Errorf("type %s is both a configuration and provider result in module %q", typeOf, provider.value.Module)
		}
		configs[typeOf] = compiled.Config{Module: declaration.Module, Path: declaration.Declaration.Path, Type: typeOf}
	}

	bindings := make(map[reflect.Type]compiled.Binding, len(collection.Bindings))
	for _, declaration := range collection.Bindings {
		contract := declaration.Declaration.Contract
		implementation := declaration.Declaration.Implementation
		if contract == nil || contract.Kind() != reflect.Interface {
			return nil, fmt.Errorf("module %q binding contract must be an interface, got %v", declaration.Module, contract)
		}
		provider, exists := providers[implementation]
		if !exists {
			return nil, fmt.Errorf("module %q binds %s to missing implementation %s", declaration.Module, contract, implementation)
		}
		if provider.value.Module != declaration.Module {
			return nil, fmt.Errorf("module %q cannot bind implementation %s owned by module %q", declaration.Module, implementation, provider.value.Module)
		}
		if !implementation.Implements(contract) {
			return nil, fmt.Errorf("implementation %s does not implement %s", implementation, contract)
		}
		if previous, exists := bindings[contract]; exists {
			return nil, fmt.Errorf("contract %s has bindings in both %q and %q", contract, previous.Module, declaration.Module)
		}
		bindings[contract] = compiled.Binding{Module: declaration.Module, Contract: contract, Implementation: implementation}
	}

	exports := make(map[reflect.Type]string, len(collection.Exports))
	for _, declaration := range collection.Exports {
		contract := declaration.Declaration.Contract
		if contract == nil || contract.Kind() != reflect.Interface {
			return nil, fmt.Errorf("module %q can only export interface contracts, got %v", declaration.Module, contract)
		}
		binding, exists := bindings[contract]
		if !exists || binding.Module != declaration.Module {
			return nil, fmt.Errorf("module %q exports contract %s without a local binding", declaration.Module, contract)
		}
		if previous, exists := exports[contract]; exists {
			return nil, fmt.Errorf("contract %s exported by both %q and %q", contract, previous, declaration.Module)
		}
		exports[contract] = declaration.Module
	}

	dependencies := make(map[*providerMeta][]*providerMeta, len(ordered))
	for _, provider := range ordered {
		constructorType := provider.value.Constructor.Type()
		for index := range constructorType.NumIn() {
			requested := constructorType.In(index)
			if requested == contextType || requested.Implements(contextType) || requested == registryType {
				return nil, fmt.Errorf("provider %s in module %q has forbidden dependency %s", provider.value.Name, provider.value.Module, requested)
			}
			if configValue, ok := configs[requested]; ok {
				if configValue.Module != provider.value.Module {
					return nil, fmt.Errorf("provider %s in module %q depends on configuration %s owned by %q", provider.value.Name, provider.value.Module, requested, configValue.Module)
				}
				provider.value.Dependencies = append(provider.value.Dependencies, compiled.Dependency{Requested: requested, Resolved: requested, Config: true})
				continue
			}
			resolved := requested
			if requested.Kind() == reflect.Interface {
				binding, ok := bindings[requested]
				if !ok {
					return nil, fmt.Errorf("provider %s in module %q has missing binding for %s", provider.value.Name, provider.value.Module, requested)
				}
				if binding.Module != provider.value.Module {
					if owner, ok := exports[requested]; !ok || owner != binding.Module {
						return nil, fmt.Errorf("provider %s in module %q depends on unexported contract %s from %q", provider.value.Name, provider.value.Module, requested, binding.Module)
					}
				}
				resolved = binding.Implementation
			}
			dependency, ok := providers[resolved]
			if !ok {
				return nil, fmt.Errorf("provider %s in module %q has missing dependency %s", provider.value.Name, provider.value.Module, requested)
			}
			if dependency.value.Module != provider.value.Module && requested.Kind() != reflect.Interface {
				return nil, fmt.Errorf("provider %s in module %q depends on private concrete type %s from %q", provider.value.Name, provider.value.Module, requested, dependency.value.Module)
			}
			provider.value.Dependencies = append(provider.value.Dependencies, compiled.Dependency{Requested: requested, Resolved: resolved})
			dependencies[provider] = append(dependencies[provider], dependency)
		}
	}

	topological, err := stableTopological(ordered, dependencies)
	if err != nil {
		return nil, err
	}
	plan := &compiled.Plan{}
	for _, item := range topological {
		plan.Providers = append(plan.Providers, item.value)
	}
	for _, binding := range bindings {
		plan.Bindings = append(plan.Bindings, binding)
	}
	sort.Slice(plan.Bindings, func(i, j int) bool { return plan.Bindings[i].Contract.String() < plan.Bindings[j].Contract.String() })
	for _, value := range configs {
		plan.Configs = append(plan.Configs, value)
	}
	sort.Slice(plan.Configs, func(i, j int) bool {
		if plan.Configs[i].Module != plan.Configs[j].Module {
			return moduleOrder(collection, plan.Configs[i].Module) < moduleOrder(collection, plan.Configs[j].Module)
		}
		return plan.Configs[i].Path < plan.Configs[j].Path
	})
	plan.Graph = buildGraph(plan)
	return plan, nil
}

func inspectProvider(declaration registration.Provider) (compiled.Provider, error) {
	if declaration.Declaration.Constructor == nil {
		return compiled.Provider{}, fmt.Errorf("module %q registered a nil provider", declaration.Module)
	}
	value := reflect.ValueOf(declaration.Declaration.Constructor)
	typeOf := value.Type()
	if typeOf.Kind() != reflect.Func {
		return compiled.Provider{}, fmt.Errorf("module %q provider must be a function, got %s", declaration.Module, typeOf)
	}
	if typeOf.IsVariadic() {
		return compiled.Provider{}, fmt.Errorf("module %q provider %s must not be variadic", declaration.Module, functionName(value))
	}
	if typeOf.NumOut() != 1 && !(typeOf.NumOut() == 2 && typeOf.Out(1) == errorType) {
		return compiled.Provider{}, fmt.Errorf("module %q provider %s must return Component or (Component, error)", declaration.Module, functionName(value))
	}
	result := typeOf.Out(0)
	if result.Kind() == reflect.Interface || result == errorType {
		return compiled.Provider{}, fmt.Errorf("module %q provider %s must return a concrete component, got %s", declaration.Module, functionName(value), result)
	}
	name := functionName(value)
	return compiled.Provider{ID: declaration.Module + ":" + result.String(), Module: declaration.Module, ModuleOrder: declaration.ModuleOrder, Order: declaration.Order, Name: name, Constructor: value, Type: result}, nil
}

func functionName(value reflect.Value) string {
	if function := runtime.FuncForPC(value.Pointer()); function != nil {
		return function.Name()
	}
	return value.Type().String()
}

func stableTopological(nodes []*providerMeta, dependencies map[*providerMeta][]*providerMeta) ([]*providerMeta, error) {
	indegree := make(map[*providerMeta]int, len(nodes))
	consumers := make(map[*providerMeta][]*providerMeta, len(nodes))
	for _, node := range nodes {
		indegree[node] = len(dependencies[node])
		for _, dependency := range dependencies[node] {
			consumers[dependency] = append(consumers[dependency], node)
		}
	}
	less := func(a, b *providerMeta) bool {
		if a.value.ModuleOrder != b.value.ModuleOrder {
			return a.value.ModuleOrder < b.value.ModuleOrder
		}
		return a.value.Order < b.value.Order
	}
	ready := make([]*providerMeta, 0)
	for _, node := range nodes {
		if indegree[node] == 0 {
			ready = append(ready, node)
		}
	}
	sort.SliceStable(ready, func(i, j int) bool { return less(ready[i], ready[j]) })
	result := make([]*providerMeta, 0, len(nodes))
	for len(ready) > 0 {
		node := ready[0]
		ready = ready[1:]
		result = append(result, node)
		for _, consumer := range consumers[node] {
			indegree[consumer]--
			if indegree[consumer] == 0 {
				ready = append(ready, consumer)
				sort.SliceStable(ready, func(i, j int) bool { return less(ready[i], ready[j]) })
			}
		}
	}
	if len(result) != len(nodes) {
		var cyclic []string
		for _, node := range nodes {
			if indegree[node] > 0 {
				cyclic = append(cyclic, node.value.ID)
			}
		}
		sort.Strings(cyclic)
		return nil, fmt.Errorf("dependency cycle detected among %s", strings.Join(cyclic, ", "))
	}
	return result, nil
}

func buildGraph(plan *compiled.Plan) di.Graph {
	graph := di.Graph{}
	configIDs := make(map[reflect.Type]string)
	for index, cfg := range plan.Configs {
		id := cfg.Module + ":config:" + cfg.Type.String()
		configIDs[cfg.Type] = id
		graph.Nodes = append(graph.Nodes, di.Node{ID: id, Module: cfg.Module, Type: cfg.Type.String(), Kind: di.ConfigNode, Order: index})
	}
	offset := len(graph.Nodes)
	for index, provider := range plan.Providers {
		graph.Nodes = append(graph.Nodes, di.Node{ID: provider.ID, Module: provider.Module, Type: provider.Type.String(), Kind: di.ProviderNode, Order: offset + index, Lifecycle: lifecycleNames(provider.Type)})
		for _, dependency := range provider.Dependencies {
			from := ""
			if dependency.Config {
				from = configIDs[dependency.Resolved]
			} else {
				from = providerID(plan, dependency.Resolved)
			}
			graph.Edges = append(graph.Edges, di.Edge{From: from, To: provider.ID, Via: dependency.Requested.String()})
		}
	}
	return graph
}

func providerID(plan *compiled.Plan, typeOf reflect.Type) string {
	for _, provider := range plan.Providers {
		if provider.Type == typeOf {
			return provider.ID
		}
	}
	return typeOf.String()
}

func lifecycleNames(typeOf reflect.Type) []string {
	checks := []struct {
		name   string
		target reflect.Type
	}{
		{"Prepare", reflect.TypeOf((*lifecycle.Preparer)(nil)).Elem()},
		{"Start", reflect.TypeOf((*lifecycle.Starter)(nil)).Elem()},
		{"Run", reflect.TypeOf((*lifecycle.Runner)(nil)).Elem()},
		{"Stop", reflect.TypeOf((*lifecycle.Stopper)(nil)).Elem()},
		{"Close", reflect.TypeOf((*lifecycle.Closer)(nil)).Elem()},
	}
	var result []string
	for _, check := range checks {
		if typeOf.Implements(check.target) {
			result = append(result, check.name)
		}
	}
	return result
}

func moduleOrder(collection registration.Collection, name string) int {
	for _, provider := range collection.Providers {
		if provider.Module == name {
			return provider.ModuleOrder
		}
	}
	for _, cfg := range collection.Configs {
		if cfg.Module == name {
			return cfg.ModuleOrder
		}
	}
	return int(^uint(0) >> 1)
}
