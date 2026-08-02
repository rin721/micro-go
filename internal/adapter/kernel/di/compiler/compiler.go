// Package compiler 把模块声明编译为稳定、可执行的依赖计划。
// 模块可见性、唯一绑定、Provider 签名和拓扑顺序均由项目治理，不能委托给 Dig。
package compiler

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiled"
	"github.com/rin721/micro-go/internal/adapter/kernel/module"
	"github.com/rin721/micro-go/internal/kernel/di"
	"github.com/rin721/micro-go/internal/kernel/lifecycle"
	"github.com/rin721/micro-go/internal/kernel/module"
)

// Compiler 是项目依赖图规则的无状态实现。
// 它被显式注入 Runtime，确保更换构造引擎不会连带改变图校验规则。
type Compiler struct{}

// New 创建依赖图编译器。
func New() *Compiler { return &Compiler{} }

// Compile 编译模块声明并返回冻结计划。
func (*Compiler) Compile(collection registration.Collection) (*compiled.Plan, error) {
	return Compile(collection)
}

// 这些 reflect.Type 是 Compiler 禁止或特殊处理的稳定语言契约类型。
var (
	// errorType 用于校验 Provider 可选的第二返回值。
	errorType = reflect.TypeOf((*error)(nil)).Elem()
	// contextType 用于禁止把运行期 Context 注入静态组件图。
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	// registryType 用于禁止业务组件在运行期取得声明注册器。
	registryType = reflect.TypeOf((*module.Registry)(nil)).Elem()
)

// providerMeta 为编译后的 Provider 保留原始收集位置，供确定性拓扑排序使用。
type providerMeta struct {
	// value 是逐步补齐依赖信息的内部 Provider 值。
	value compiled.Provider
	// index 是 Provider 在完整收集序列中的位置。
	index int
}

// Compile 完成纯静态图检查，不构造组件，也不保留第三方容器。
// 成功返回的 Providers 已是确定顺序，Build 可以据此逐个实例化并登记资源。
func Compile(collection registration.Collection) (*compiled.Plan, error) {
	// 具体返回类型是全局唯一键；重复 Provider 必须在调用第三方容器前给出项目错误。
	providers := make(map[reflect.Type]*providerMeta, len(collection.Providers))
	ordered := make([]*providerMeta, 0, len(collection.Providers))
	// 第一遍只检查 Provider 自身签名和结果唯一性，不解析参数依赖。
	for index, declaration := range collection.Providers {
		provider, err := inspectProvider(declaration)
		if err != nil {
			return nil, err
		}
		if previous, exists := providers[provider.Type]; exists {
			return nil, fmt.Errorf("duplicate provider result %s in modules %q and %q", provider.Type, previous.value.Module, provider.Module)
		}
		// meta 同时进入类型索引和声明顺序切片，两处始终指向同一可补充对象。
		meta := &providerMeta{value: provider, index: index}
		providers[provider.Type] = meta
		ordered = append(ordered, meta)
	}

	// 配置也按 Go 类型唯一，并且不能同时由 Provider 构造，否则所有权来源不明确。
	configs := make(map[reflect.Type]compiled.Config, len(collection.Configs))
	// 配置声明先于依赖解析完成校验，Provider 参数才能准确区分配置和组件。
	for _, declaration := range collection.Configs {
		typeOf := declaration.Declaration.Type
		if typeOf == nil || typeOf.Kind() != reflect.Struct {
			return nil, fmt.Errorf("module %q configuration must be a struct value, got %v", declaration.Module, typeOf)
		}
		// 空路径会让配置所有权覆盖整棵树，因此必须显式拒绝。
		if strings.TrimSpace(declaration.Declaration.Path) == "" {
			return nil, fmt.Errorf("module %q configuration %s has an empty path", declaration.Module, typeOf)
		}
		// 一个类型只能由一个模块拥有，防止同型配置在不同路径产生注入歧义。
		if previous, exists := configs[typeOf]; exists {
			return nil, fmt.Errorf("configuration %s declared by both %q and %q", typeOf, previous.Module, declaration.Module)
		}
		// 配置值由 Loader 产生，不允许同型 Provider 再创建第二个来源。
		if provider, exists := providers[typeOf]; exists {
			return nil, fmt.Errorf("type %s is both a configuration and provider result in module %q", typeOf, provider.value.Module)
		}
		configs[typeOf] = compiled.Config{Module: declaration.Module, Path: declaration.Declaration.Path, Type: typeOf}
	}

	// Binding 只能引用本模块拥有的具体实现，防止模块替别人导出或改写契约。
	bindings := make(map[reflect.Type]compiled.Binding, len(collection.Bindings))
	// Binding 校验在 Export 之前完成，后者才能只引用已经成立的本地接口别名。
	for _, declaration := range collection.Bindings {
		contract := declaration.Declaration.Contract
		implementation := declaration.Declaration.Implementation
		if contract == nil || contract.Kind() != reflect.Interface {
			return nil, fmt.Errorf("module %q binding contract must be an interface, got %v", declaration.Module, contract)
		}
		// 实现必须已有具体 Provider，Binding 自身不会创建实例。
		provider, exists := providers[implementation]
		if !exists {
			return nil, fmt.Errorf("module %q binds %s to missing implementation %s", declaration.Module, contract, implementation)
		}
		if provider.value.Module != declaration.Module {
			return nil, fmt.Errorf("module %q cannot bind implementation %s owned by module %q", declaration.Module, implementation, provider.value.Module)
		}
		// reflect.Implements 在静态阶段验证方法集，避免把类型错误留给 Dig。
		if !implementation.Implements(contract) {
			return nil, fmt.Errorf("implementation %s does not implement %s", implementation, contract)
		}
		// 每个接口全局只能选择一个实现，调用方无需处理命名或优先级。
		if previous, exists := bindings[contract]; exists {
			return nil, fmt.Errorf("contract %s has bindings in both %q and %q", contract, previous.Module, declaration.Module)
		}
		bindings[contract] = compiled.Binding{Module: declaration.Module, Contract: contract, Implementation: implementation}
	}

	// Export 只对接口生效；具体类型始终是模块私有实现细节。
	exports := make(map[reflect.Type]string, len(collection.Exports))
	// Export 只改变跨模块可见性，不改变 Binding 的实例或所有者。
	for _, declaration := range collection.Exports {
		contract := declaration.Declaration.Contract
		if contract == nil || contract.Kind() != reflect.Interface {
			return nil, fmt.Errorf("module %q can only export interface contracts, got %v", declaration.Module, contract)
		}
		// 模块只能导出自己已经绑定的契约，不能替其他模块转授权限。
		binding, exists := bindings[contract]
		if !exists || binding.Module != declaration.Module {
			return nil, fmt.Errorf("module %q exports contract %s without a local binding", declaration.Module, contract)
		}
		if previous, exists := exports[contract]; exists {
			return nil, fmt.Errorf("contract %s exported by both %q and %q", contract, previous, declaration.Module)
		}
		exports[contract] = declaration.Module
	}

	// 依赖解析同时执行配置所有权和跨模块可见性检查，形成项目自己的有向图。
	dependencies := make(map[*providerMeta][]*providerMeta, len(ordered))
	// Provider 按声明顺序解析参数，Dependencies 切片因此与构造函数参数严格对齐。
	for _, provider := range ordered {
		constructorType := provider.value.Constructor.Type()
		for index := range constructorType.NumIn() {
			// Context 和 Registry 都属于运行期控制面，注入组件会隐藏生命周期或图修改能力。
			requested := constructorType.In(index)
			if requested == contextType || requested.Implements(contextType) || requested == registryType {
				return nil, fmt.Errorf("provider %s in module %q has forbidden dependency %s", provider.value.Name, provider.value.Module, requested)
			}
			if configValue, ok := configs[requested]; ok {
				// 配置只能注入所有者模块，阻止跨模块读取内部配置结构。
				if configValue.Module != provider.value.Module {
					return nil, fmt.Errorf("provider %s in module %q depends on configuration %s owned by %q", provider.value.Name, provider.value.Module, requested, configValue.Module)
				}
				provider.value.Dependencies = append(provider.value.Dependencies, compiled.Dependency{Requested: requested, Resolved: requested, Config: true})
				continue
			}
			// 具体类型直接解析；接口必须先经过唯一 Binding 映射到实现类型。
			resolved := requested
			if requested.Kind() == reflect.Interface {
				binding, ok := bindings[requested]
				if !ok {
					return nil, fmt.Errorf("provider %s in module %q has missing binding for %s", provider.value.Name, provider.value.Module, requested)
				}
				if binding.Module != provider.value.Module {
					// 跨模块接口依赖只有在所有者显式 Export 后才可见。
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
				// 具体实现永远是模块私有；跨模块依赖必须回到公开接口。
				return nil, fmt.Errorf("provider %s in module %q depends on private concrete type %s from %q", provider.value.Name, provider.value.Module, requested, dependency.value.Module)
			}
			provider.value.Dependencies = append(provider.value.Dependencies, compiled.Dependency{Requested: requested, Resolved: resolved})
			dependencies[provider] = append(dependencies[provider], dependency)
		}
	}
	if err := validateModuleGraph(ordered, dependencies); err != nil {
		return nil, err
	}

	// 先稳定排序再交给 Dig；Dig 只负责调用构造函数，不决定项目的实例化次序。
	topological, err := stableTopological(ordered, dependencies)
	if err != nil {
		return nil, err
	}
	plan := &compiled.Plan{}
	// 拓扑结果已经稳定，复制值而不再暴露可变 providerMeta。
	for _, item := range topological {
		plan.Providers = append(plan.Providers, item.value)
	}
	// map 遍历无序，因此 Binding 在写入 Plan 后按契约全名排序。
	for _, binding := range bindings {
		plan.Bindings = append(plan.Bindings, binding)
	}
	sort.Slice(plan.Bindings, func(i, j int) bool { return plan.Bindings[i].Contract.String() < plan.Bindings[j].Contract.String() })
	// 配置先按模块输入顺序，再按模块内 Path 排序，保证快照和诊断稳定。
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

// validateModuleGraph 把 Provider 依赖折叠到模块级，并拒绝模块职责的双向循环。
func validateModuleGraph(providers []*providerMeta, dependencies map[*providerMeta][]*providerMeta) error {
	// Provider 图无环不代表模块图无环：两个模块可以分别通过不同 Provider 互相依赖，
	// 因此必须折叠为模块依赖后再次校验，避免包级职责形成双向耦合。
	edges := make(map[string]map[string]struct{})
	modules := make(map[string]struct{})
	// 同模块边不进入模块图；跨模块边记录为消费者到依赖的方向。
	for _, provider := range providers {
		consumerModule := provider.value.Module
		modules[consumerModule] = struct{}{}
		for _, dependency := range dependencies[provider] {
			dependencyModule := dependency.value.Module
			modules[dependencyModule] = struct{}{}
			if consumerModule == dependencyModule {
				continue
			}
			// 首次看到消费者时延迟创建集合，随后用空 struct 去重依赖模块。
			if edges[consumerModule] == nil {
				edges[consumerModule] = make(map[string]struct{})
			}
			edges[consumerModule][dependencyModule] = struct{}{}
		}
	}

	// 名称排序让 DFS 起点和错误文本不受 map 顺序影响。
	names := make([]string, 0, len(modules))
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)
	state := make(map[string]uint8, len(names))
	stack := make([]string, 0, len(names))
	// visit 使用 0/1/2 表示未访问、当前路径、已完成；返回非空切片即发现环。
	var visit func(string) []string
	visit = func(name string) []string {
		// 当前模块进入递归栈，后续遇到 state=1 的依赖即可截取环区间。
		state[name] = 1
		stack = append(stack, name)
		dependencies := make([]string, 0, len(edges[name]))
		for dependency := range edges[name] {
			dependencies = append(dependencies, dependency)
		}
		// 依赖排序保证递归选择和首个报告的环稳定。
		sort.Strings(dependencies)
		for _, dependency := range dependencies {
			switch state[dependency] {
			case 0:
				if cycle := visit(dependency); len(cycle) > 0 {
					return cycle
				}
			case 1:
				// 从依赖第一次出现的位置截取当前递归栈，得到组成环的模块集合。
				start := 0
				for index, item := range stack {
					if item == dependency {
						start = index
						break
					}
				}
				cycle := append([]string(nil), stack[start:]...)
				// 错误强调参与者而非遍历方向，因此按名称排序后返回。
				sort.Strings(cycle)
				return cycle
			}
		}
		// 正常退出当前路径并标记完成，后续起点无需重复遍历。
		stack = stack[:len(stack)-1]
		state[name] = 2
		return nil
	}
	for _, name := range names {
		// 已被先前 DFS 覆盖的模块不再作为新起点。
		if state[name] != 0 {
			continue
		}
		if cycle := visit(name); len(cycle) > 0 {
			return fmt.Errorf("module dependency cycle detected among %s", strings.Join(cycle, ", "))
		}
	}
	return nil
}

// inspectProvider 校验单个构造函数签名并提取稳定诊断元数据。
func inspectProvider(declaration registration.Provider) (compiled.Provider, error) {
	// 只允许普通函数返回 Concrete 或 (Concrete, error)，主动排除 dig.In/Out、集合和命名注入。
	if declaration.Declaration.Constructor == nil {
		return compiled.Provider{}, fmt.Errorf("module %q registered a nil provider", declaration.Module)
	}
	// reflect.Value 保留稍后交给构造引擎的真实函数，Type 用于完成纯静态检查。
	value := reflect.ValueOf(declaration.Declaration.Constructor)
	typeOf := value.Type()
	if typeOf.Kind() != reflect.Func {
		return compiled.Provider{}, fmt.Errorf("module %q provider must be a function, got %s", declaration.Module, typeOf)
	}
	// 变参构造的依赖数量不固定，不符合冻结静态图要求。
	if typeOf.IsVariadic() {
		return compiled.Provider{}, fmt.Errorf("module %q provider %s must not be variadic", declaration.Module, functionName(value))
	}
	// 只接受一个组件，或组件加 error；其他返回组合没有项目语义。
	if typeOf.NumOut() != 1 && !(typeOf.NumOut() == 2 && typeOf.Out(1) == errorType) {
		return compiled.Provider{}, fmt.Errorf("module %q provider %s must return Component or (Component, error)", declaration.Module, functionName(value))
	}
	// 结果必须是具体组件，接口选择只能通过显式 Binding 声明。
	result := typeOf.Out(0)
	if result.Kind() == reflect.Interface || result == errorType {
		return compiled.Provider{}, fmt.Errorf("module %q provider %s must return a concrete component, got %s", declaration.Module, functionName(value), result)
	}
	// 函数名和模块、结果类型共同形成可诊断的冻结 Provider。
	name := functionName(value)
	return compiled.Provider{ID: declaration.Module + ":" + result.String(), Module: declaration.Module, ModuleOrder: declaration.ModuleOrder, Order: declaration.Order, Name: name, Constructor: value, Type: result}, nil
}

// functionName 优先返回运行期符号名，无法定位时回退为稳定函数类型字符串。
func functionName(value reflect.Value) string {
	// 普通已编译函数可以由程序计数器解析到包限定名称。
	if function := runtime.FuncForPC(value.Pointer()); function != nil {
		return function.Name()
	}
	// 动态函数等没有符号表条目时仍提供可读签名。
	return value.Type().String()
}

// stableTopological 使用稳定 Kahn 算法生成依赖在前、消费者在后的 Provider 顺序。
func stableTopological(nodes []*providerMeta, dependencies map[*providerMeta][]*providerMeta) ([]*providerMeta, error) {
	// Kahn 算法的 ready 集合每次按模块顺序、模块内声明顺序排序，使多个合法拓扑序
	// 收敛为一个可测试、可导出的确定结果。
	indegree := make(map[*providerMeta]int, len(nodes))
	consumers := make(map[*providerMeta][]*providerMeta, len(nodes))
	// 入度直接取依赖数量，同时建立依赖到消费者的反向索引供出队更新。
	for _, node := range nodes {
		indegree[node] = len(dependencies[node])
		for _, dependency := range dependencies[node] {
			consumers[dependency] = append(consumers[dependency], node)
		}
	}
	// less 只使用用户可控的模块顺序和声明顺序，不依赖指针或 map 地址。
	less := func(a, b *providerMeta) bool {
		if a.value.ModuleOrder != b.value.ModuleOrder {
			return a.value.ModuleOrder < b.value.ModuleOrder
		}
		return a.value.Order < b.value.Order
	}
	// 初始 ready 集合包含所有无依赖 Provider，并在第一次消费前稳定排序。
	ready := make([]*providerMeta, 0)
	for _, node := range nodes {
		if indegree[node] == 0 {
			ready = append(ready, node)
		}
	}
	sort.SliceStable(ready, func(i, j int) bool { return less(ready[i], ready[j]) })
	result := make([]*providerMeta, 0, len(nodes))
	// 每轮取最小稳定节点；新入度归零的消费者重新加入并排序。
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
		// 未被消费的剩余节点属于至少一个环；排序 ID 后错误文本也保持稳定。
		var cyclic []string
		for _, node := range nodes {
			if indegree[node] > 0 {
				cyclic = append(cyclic, node.value.ID)
			}
		}
		sort.Strings(cyclic)
		return nil, fmt.Errorf("dependency cycle detected among %s", strings.Join(cyclic, ", "))
	}
	// 所有节点均被消费代表依赖图无环。
	return result, nil
}

// buildGraph 把内部反射计划投影为不含构造函数和实例的项目只读图。
func buildGraph(plan *compiled.Plan) di.Graph {
	// 公共 Graph 只复制字符串和值，不携带 reflect.Value、Provider 函数或 Dig 容器。
	graph := di.Graph{}
	configIDs := make(map[reflect.Type]string)
	// 配置节点先出现并记录类型到 ID 的映射，Provider 边稍后可以直接引用。
	for index, cfg := range plan.Configs {
		id := cfg.Module + ":config:" + cfg.Type.String()
		configIDs[cfg.Type] = id
		graph.Nodes = append(graph.Nodes, di.Node{ID: id, Module: cfg.Module, Type: cfg.Type.String(), Kind: di.ConfigNode, Order: index})
	}
	// Provider 节点顺序接在配置节点之后，保持全图 Order 唯一递增。
	offset := len(graph.Nodes)
	for index, provider := range plan.Providers {
		graph.Nodes = append(graph.Nodes, di.Node{ID: provider.ID, Module: provider.Module, Type: provider.Type.String(), Kind: di.ProviderNode, Order: offset + index, Lifecycle: lifecycleNames(provider.Type)})
		for _, dependency := range provider.Dependencies {
			// 配置依赖使用预先生成的配置 ID，组件依赖通过结果类型查找 Provider ID。
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

// providerID 根据具体结果类型查找冻结计划中的 Provider 节点 ID。
func providerID(plan *compiled.Plan, typeOf reflect.Type) string {
	// Provider 数量有限且 Plan 已排序，线性查找保持实现简单并无额外可变索引。
	for _, provider := range plan.Providers {
		if provider.Type == typeOf {
			return provider.ID
		}
	}
	// 理论上 Compiler 已保证依赖存在；回退类型名让异常图仍可被诊断而非产生空 ID。
	return typeOf.String()
}

// lifecycleNames 返回类型静态实现的可选生命周期接口名称。
func lifecycleNames(typeOf reflect.Type) []string {
	// 生命周期能力在编译期只用于诊断展示，真正调用仍在实例构造后通过接口断言完成。
	checks := []struct {
		// name 是写入公共 Graph 的稳定能力名称。
		name string
		// target 是用于 reflect.Implements 的 Kernel 小接口类型。
		target reflect.Type
	}{
		{"Prepare", reflect.TypeOf((*lifecycle.Preparer)(nil)).Elem()},
		{"Start", reflect.TypeOf((*lifecycle.Starter)(nil)).Elem()},
		{"Run", reflect.TypeOf((*lifecycle.Runner)(nil)).Elem()},
		{"Stop", reflect.TypeOf((*lifecycle.Stopper)(nil)).Elem()},
		{"Close", reflect.TypeOf((*lifecycle.Closer)(nil)).Elem()},
	}
	// 结果沿固定 checks 顺序追加，不需要额外排序。
	var result []string
	for _, check := range checks {
		if typeOf.Implements(check.target) {
			result = append(result, check.name)
		}
	}
	return result
}

// moduleOrder 从原始 Collection 中恢复模块输入顺序，供配置稳定排序使用。
func moduleOrder(collection registration.Collection, name string) int {
	// 优先从 Provider 找模块；同一模块的所有声明共享 ModuleOrder。
	for _, provider := range collection.Providers {
		if provider.Module == name {
			return provider.ModuleOrder
		}
	}
	// 没有 Provider 的纯配置模块仍能从 Config 声明取得顺序。
	for _, cfg := range collection.Configs {
		if cfg.Module == name {
			return cfg.ModuleOrder
		}
	}
	// 理论上所有编译配置都来自 Collection；最大 int 将异常项稳定排到末尾。
	return int(^uint(0) >> 1)
}
