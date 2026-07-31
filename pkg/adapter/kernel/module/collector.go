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
	module      string
	moduleOrder int
	collection  *Collection
	frozen      bool
	order       int
}

// New 创建绑定到单个模块和共享 Collection 的 Registry。
func New(moduleName string, moduleOrder int, collection *Collection) *Registry {
	return &Registry{module: moduleName, moduleOrder: moduleOrder, collection: collection}
}

// Freeze 永久关闭该模块的声明窗口，避免组件保存 Registry 后在运行期修改图。
func (r *Registry) Freeze() { r.frozen = true }

// RegisterProvider 记录 Provider 并分配模块内稳定序号。
func (r *Registry) RegisterProvider(value module.ProviderDeclaration) error {
	if err := r.mutable(); err != nil {
		return err
	}
	r.collection.Providers = append(r.collection.Providers, Provider{r.module, r.moduleOrder, r.next(), value})
	return nil
}

// RegisterBinding 记录接口到实现的绑定声明。
func (r *Registry) RegisterBinding(value module.BindingDeclaration) error {
	if err := r.mutable(); err != nil {
		return err
	}
	r.collection.Bindings = append(r.collection.Bindings, Binding{r.module, r.moduleOrder, r.next(), value})
	return nil
}

// RegisterExport 记录允许跨模块使用的接口契约。
func (r *Registry) RegisterExport(value module.ExportDeclaration) error {
	if err := r.mutable(); err != nil {
		return err
	}
	r.collection.Exports = append(r.collection.Exports, Export{r.module, r.moduleOrder, r.next(), value})
	return nil
}

// RegisterConfig 记录模块拥有的配置类型和路径。
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

// Collect 按传入顺序执行模块注册并冻结各 Registry。
// nil、空名、重名、Register error 和 panic 都在构造任何组件前失败。
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
		// 为每个 Module.Register 单独建立 panic 边界，确保错误能标注准确模块名。
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
