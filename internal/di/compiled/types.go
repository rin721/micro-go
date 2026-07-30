// Package compiled 定义 Registry 声明经静态校验后的内部执行计划。
// 这些类型包含 reflect.Value，因此必须留在 internal，不能成为用户扩展契约。
package compiled

import (
	"reflect"

	"github.com/rin721/micro-go/kernel/di"
)

// Dependency 同时保留 Provider 请求的类型和 Binding 后实际解析的类型。
type Dependency struct {
	// Requested 是 Provider 参数声明的原始类型。
	Requested reflect.Type
	// Resolved 是应用 Binding 后实际提供实例的具体类型。
	Resolved reflect.Type
	// Config 标记该依赖来自强类型配置而不是 Provider。
	Config bool
}

// Provider 是已经验证过签名、所有权和依赖的构造节点。
type Provider struct {
	// ID 是模块名与结果类型组成的稳定节点标识。
	ID string
	// Module 是声明 Provider 的模块名。
	Module string
	// ModuleOrder 是模块在 Application Options 中的顺序。
	ModuleOrder int
	// Order 是 Provider 在模块 Register 中的声明顺序。
	Order int
	// Name 是用于诊断的构造函数名称。
	Name string
	// Constructor 是已验证的反射函数值。
	Constructor reflect.Value
	// Type 是构造函数的具体结果类型。
	Type reflect.Type
	// Dependencies 是按构造参数顺序解析的依赖。
	Dependencies []Dependency
}

// Binding 是已验证的接口别名；执行引擎必须让接口与具体类型指向同一实例。
type Binding struct {
	// Module 是拥有该接口绑定的模块。
	Module string
	// Contract 是消费者依赖的接口类型。
	Contract reflect.Type
	// Implementation 是提供同一实例的具体类型。
	Implementation reflect.Type
}

// Config 是已验证的模块配置声明。
type Config struct {
	// Module 是配置所有者模块。
	Module string
	// Path 是合并配置树中的读取路径。
	Path string
	// Type 是配置 struct 类型。
	Type reflect.Type
}

// Plan 是构造阶段唯一接受的冻结计划，Providers 已按稳定拓扑顺序排列。
type Plan struct {
	// Providers 已按稳定拓扑顺序排列。
	Providers []Provider
	// Bindings 已按契约类型名稳定排列。
	Bindings []Binding
	// Configs 已按模块及路径稳定排列。
	Configs []Config
	// Graph 是对外可导出的项目图模型。
	Graph di.Graph
}

// Instance 把已构造值与其 Provider 元数据配对，供生命周期和错误诊断使用。
type Instance struct {
	// Provider 提供生命周期顺序和错误上下文。
	Provider Provider
	// Value 是已经成功构造的普通 Go 实例。
	Value any
}
