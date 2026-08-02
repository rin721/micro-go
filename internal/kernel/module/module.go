// Package module 定义 Kernel 模块注册契约和泛型声明辅助函数。
package module

import (
	"fmt"
	"reflect"
)

// Module 是应用组装时的最小注册单元。
//
// Name 必须在一次编译中保持唯一；Register 只用于声明 Provider、Binding、Export
// 和配置，不允许在运行期查询对象。这样可以把依赖关系冻结在启动阶段，避免业务代码
// 退化为 Service Locator。
type Module interface {
	// Name 返回本次依赖图编译中唯一且稳定的模块名。
	Name() string
	// Register 只向 Registry 声明静态事实，不得解析运行期实例。
	Register(Registry) error
}

// ProviderDeclaration 保存一个构造函数声明。构造函数是否合法由图编译器统一校验，
// Registry 不提前解释反射签名，避免注册层和编译层出现两套规则。
type ProviderDeclaration struct {
	// Constructor 是用户提供的普通 Go 构造函数。
	Constructor any
}

// BindingDeclaration 描述“接口契约由哪个具体类型实现”。Binding 只是同一实例的别名，
// 不会额外创建实现对象。
type BindingDeclaration struct {
	// Contract 是调用方依赖的接口类型。
	Contract reflect.Type
	// Implementation 是当前模块 Provider 返回的具体类型。
	Implementation reflect.Type
}

// ExportDeclaration 声明允许其他模块依赖的接口契约。
// 只导出接口而不导出具体类型，是模块之间保持低耦合的关键边界。
type ExportDeclaration struct {
	// Contract 是允许跨模块依赖的接口类型。
	Contract reflect.Type
}

// ConfigDeclaration 把强类型配置及其点分路径归属到当前模块。
type ConfigDeclaration struct {
	// Type 是配置 struct 的非指针类型。
	Type reflect.Type
	// Path 是 Source 合并树中的点分读取路径。
	Path string
}

// Registry 是模块注册期间唯一可见的写入接口。
// Runtime 会在 Register 返回后冻结其内部实现，因此模块不能在运行期改变依赖图。
type Registry interface {
	// RegisterProvider 登记一个普通 Go 构造函数。
	RegisterProvider(ProviderDeclaration) error
	// RegisterBinding 登记接口到具体实现的同实例别名。
	RegisterBinding(BindingDeclaration) error
	// RegisterExport 公开允许其他模块请求的接口契约。
	RegisterExport(ExportDeclaration) error
	// RegisterConfig 登记当前模块拥有的强类型配置。
	RegisterConfig(ConfigDeclaration) error
}

// Provide 为当前模块登记一个构造函数。
// 使用 any 是为了让业务构造函数保持普通 Go 签名，具体约束由 Compile 阶段集中报告。
func Provide(registry Registry, constructor any) error {
	// nil Registry 表示组合根装配错误，必须在反射处理前直接拒绝。
	if registry == nil {
		return fmt.Errorf("module registry is nil")
	}
	// 构造函数原样交给注册器，签名规则由唯一的 Compiler 权威校验。
	return registry.RegisterProvider(ProviderDeclaration{Constructor: constructor})
}

// Bind 将 Implementation 绑定到 Contract；Contract 必须是接口，Implementation 必须由
// 当前模块的 Provider 提供。泛型参数让调用处无需手写 reflect.Type。
func Bind[Contract, Implementation any](registry Registry) error {
	// 所有泛型辅助函数保持相同的 nil 边界，避免调用 nil 接口方法导致 panic。
	if registry == nil {
		return fmt.Errorf("module registry is nil")
	}
	// 从 nil 指针只提取类型元数据，不创建 Contract 或 Implementation 的值。
	contract := reflect.TypeOf((*Contract)(nil)).Elem()
	implementation := reflect.TypeOf((*Implementation)(nil)).Elem()
	// 注册器保留模块归属，Compiler 再统一检查接口性、Provider 和可见性。
	return registry.RegisterBinding(BindingDeclaration{Contract: contract, Implementation: implementation})
}

// Export 公开当前模块已绑定的接口契约，使其他模块可以通过接口依赖它。
func Export[Contract any](registry Registry) error {
	// nil Registry 无法承载跨模块公开声明，因此显式返回错误。
	if registry == nil {
		return fmt.Errorf("module registry is nil")
	}
	// 泛型参数只用于得到稳定 reflect.Type 键。
	contract := reflect.TypeOf((*Contract)(nil)).Elem()
	// 是否已绑定以及是否属于当前模块由 Compiler 统一判定。
	return registry.RegisterExport(ExportDeclaration{Contract: contract})
}

// Config 声明当前模块拥有的强类型配置 T 及其读取路径。
// 配置所有权跟随模块边界，能够阻止组件绕过契约读取其他模块的内部配置。
func Config[T any](registry Registry, path string) error {
	// nil Registry 表示模块注册入口使用错误，不能静默忽略配置声明。
	if registry == nil {
		return fmt.Errorf("module registry is nil")
	}
	// T 的准确类型与点分路径共同构成 Loader 的强类型解码声明。
	typeOf := reflect.TypeOf((*T)(nil)).Elem()
	return registry.RegisterConfig(ConfigDeclaration{Type: typeOf, Path: path})
}
