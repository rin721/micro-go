// Package registration 把各 Module 的声明收集为带稳定顺序和所有权信息的中间模型。
// 它只负责记录，不解释 Provider 签名；架构规则由后续 Compiler 集中校验。
package registration

import (
	"fmt"
	"strings"

	"github.com/rin721/micro-go/internal/kernel/diagnostic"
	"github.com/rin721/micro-go/internal/kernel/module"
)

// Collector 是无状态的模块声明收集器。
// 使用实例接口让 Runtime 由 Bootstrap 注入具体实现，而不是在 Kernel 执行路径中硬编码。
type Collector struct{}

// NewCollector 创建模块声明收集器。
func NewCollector() *Collector { return &Collector{} }

// Collect 执行模块注册并冻结每个 Registry。
func (*Collector) Collect(modules []module.Module) (Collection, error) { return Collect(modules) }

// Provider 为公共声明补充所属模块及注册顺序。
type Provider struct {
	// Module 是声明所有者。
	Module string
	// ModuleOrder 是模块输入顺序。
	ModuleOrder int
	// Order 是该模块内的声明顺序。
	Order int
	// Declaration 是未解释的公共 Provider 声明。
	Declaration module.ProviderDeclaration
}

// Binding 为接口绑定补充所属模块及注册顺序。
type Binding struct {
	// Module 是声明所有者。
	Module string
	// ModuleOrder 是模块输入顺序。
	ModuleOrder int
	// Order 是该模块内的声明顺序。
	Order int
	// Declaration 是未校验的公共 Binding 声明。
	Declaration module.BindingDeclaration
}

// Export 为公开契约补充所属模块及注册顺序。
type Export struct {
	// Module 是声明所有者。
	Module string
	// ModuleOrder 是模块输入顺序。
	ModuleOrder int
	// Order 是该模块内的声明顺序。
	Order int
	// Declaration 是未校验的公共 Export 声明。
	Declaration module.ExportDeclaration
}

// Config 为配置声明补充所属模块及注册顺序。
type Config struct {
	// Module 是声明所有者。
	Module string
	// ModuleOrder 是模块输入顺序。
	ModuleOrder int
	// Order 是该模块内的声明顺序。
	Order int
	// Declaration 是未校验的公共 Config 声明。
	Declaration module.ConfigDeclaration
}

// Collection 是模块注册阶段的完整结果。各切片保留用户声明顺序，供稳定编译使用。
type Collection struct {
	// Providers 按模块及注册顺序保存 Provider。
	Providers []Provider
	// Bindings 按模块及注册顺序保存 Binding。
	Bindings []Binding
	// Exports 按模块及注册顺序保存 Export。
	Exports []Export
	// Configs 按模块及注册顺序保存 Config。
	Configs []Config
}

// Registry 是 module.Registry 的内部有状态实现，每个模块获得独立实例。
// module 和 moduleOrder 固化所有权，frozen 阻止 Register 返回后的延迟注册。
type Registry struct {
	// module 固定所有后续声明的所有者名称。
	module string
	// moduleOrder 固定模块在组合根输入中的顺序。
	moduleOrder int
	// collection 是本次 Collect 调用独占的汇总目标。
	collection *Collection
	// frozen 在 Register 返回后永久阻止继续写入。
	frozen bool
	// order 为当前模块的每条声明分配递增序号。
	order int
}

// New 创建绑定到单个模块和共享 Collection 的 Registry。
func New(moduleName string, moduleOrder int, collection *Collection) *Registry {
	// Registry 不复制 Collection，所有模块依次向同一本次调用结果写入。
	return &Registry{module: moduleName, moduleOrder: moduleOrder, collection: collection}
}

// Freeze 永久关闭该模块的声明窗口，避免组件保存 Registry 后在运行期修改图。
func (r *Registry) Freeze() {
	// frozen 只会从 false 单向变为 true，不提供恢复注册窗口的入口。
	r.frozen = true
}

// RegisterProvider 记录 Provider 并分配模块内稳定序号。
func (r *Registry) RegisterProvider(value module.ProviderDeclaration) error {
	// 每种声明先检查冻结状态，确保失败时 Collection 和序号都不变化。
	if err := r.mutable(); err != nil {
		return err
	}
	// next 在写入同一条声明时分配序号，保持跨声明种类的模块内总顺序。
	r.collection.Providers = append(r.collection.Providers, Provider{r.module, r.moduleOrder, r.next(), value})
	return nil
}

// RegisterBinding 记录接口到实现的绑定声明。
func (r *Registry) RegisterBinding(value module.BindingDeclaration) error {
	// 冻结后的延迟 Binding 会被明确拒绝而不是污染已编译图。
	if err := r.mutable(); err != nil {
		return err
	}
	r.collection.Bindings = append(r.collection.Bindings, Binding{r.module, r.moduleOrder, r.next(), value})
	return nil
}

// RegisterExport 记录允许跨模块使用的接口契约。
func (r *Registry) RegisterExport(value module.ExportDeclaration) error {
	// Export 与其他声明共享同一个模块内递增序号。
	if err := r.mutable(); err != nil {
		return err
	}
	r.collection.Exports = append(r.collection.Exports, Export{r.module, r.moduleOrder, r.next(), value})
	return nil
}

// RegisterConfig 记录模块拥有的配置类型和路径。
func (r *Registry) RegisterConfig(value module.ConfigDeclaration) error {
	// 配置所有权只能在 Module.Register 的同步窗口内声明。
	if err := r.mutable(); err != nil {
		return err
	}
	r.collection.Configs = append(r.collection.Configs, Config{r.module, r.moduleOrder, r.next(), value})
	return nil
}

// mutable 检查 Registry 是否仍处于当前 Module.Register 的声明窗口。
func (r *Registry) mutable() error {
	// frozen 后返回带模块名的稳定错误，便于定位保存 Registry 的违规模块。
	if r.frozen {
		return fmt.Errorf("registry for module %q is frozen", r.module)
	}
	return nil
}

// next 返回当前序号并推进计数器，只能在 mutable 成功后调用。
func (r *Registry) next() int {
	// 先保存返回值再递增，使第一条声明的序号从零开始。
	value := r.order
	r.order++
	return value
}

// Collect 按传入顺序执行模块注册并冻结各 Registry。
// nil、空名、重名、Register error 和 panic 都在构造任何组件前失败。
func Collect(modules []module.Module) (collection Collection, err error) {
	// seen 在调用任何 Register 前后持续记录已接受名称，拒绝重名模块。
	seen := make(map[string]struct{}, len(modules))
	// 输入下标同时成为稳定 ModuleOrder。
	for index, current := range modules {
		// nil Module 无法提供名称或声明，作为组合根错误立即返回。
		if current == nil {
			return collection, fmt.Errorf("module at index %d is nil", index)
		}
		// TrimSpace 只用于校验和规范化所有权名，后续模型统一使用清理后的值。
		name := strings.TrimSpace(current.Name())
		if name == "" {
			return collection, fmt.Errorf("module at index %d has an empty name", index)
		}
		if _, exists := seen[name]; exists {
			return collection, fmt.Errorf("duplicate module name %q", name)
		}
		// 名称通过后立即登记，后续模块即使当前注册失败也不会继续执行。
		seen[name] = struct{}{}
		registry := New(name, index, &collection)
		// 为每个 Module.Register 单独建立 panic 边界，确保错误能标注准确模块名。
		func() {
			// recover 只包围当前模块同步 Register，准确捕获并归一化用户 panic。
			defer func() {
				if value := recover(); value != nil {
					err = fmt.Errorf("register module %q: %w", name, diagnostic.NewPanicError(value))
				}
			}()
			// 普通返回错误同样补充模块名并保留原因链。
			if registerErr := current.Register(registry); registerErr != nil {
				err = fmt.Errorf("register module %q: %w", name, registerErr)
			}
		}()
		// 无论 Register 成功、返回错误还是 panic，退出同步调用后都永久冻结 Registry。
		registry.Freeze()
		if err != nil {
			return collection, err
		}
	}
	// 全部模块成功后返回按用户输入和声明顺序冻结的 Collection。
	return collection, nil
}
