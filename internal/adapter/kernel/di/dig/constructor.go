// Package digadapter 使用 Dig 执行已经由项目 Compiler 验证的构造计划。
// Dig 仅在 Build 阶段短暂存在，不进入运行期，也不暴露给业务 Provider。
package digadapter

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiled"
	"github.com/rin721/micro-go/internal/kernel/diagnostic"
	"go.uber.org/dig"
)

// Engine 是无状态的 Dig 构造引擎实现。
type Engine struct{}

// New 创建构造引擎。每次 Construct 都创建新容器，避免应用之间共享解析状态。
func New() *Engine { return &Engine{} }

// Construct 按项目 Plan 的拓扑顺序逐个取出实例。失败时仍返回此前完成的实例，
// 由 Runtime 使用独立的有界清理 Context 统一回滚，避免构造 Context 已取消后跳过释放。
func (*Engine) Construct(ctx context.Context, plan *compiled.Plan, configs map[reflect.Type]reflect.Value) ([]compiled.Instance, error) {
	// RecoverFromPanics 只是捕获 Provider panic 的执行机制，错误最终仍归一化为项目 PanicError。
	container := dig.New(dig.DeferAcyclicVerification(), dig.RecoverFromPanics())
	// 每个配置值包装为零参数函数，保持用户 Provider 仍然通过普通参数接收强类型配置。
	for typeOf, value := range configs {
		// 反射生成准确返回配置类型的零参数函数，避免把 map 或 reflect.Value 注入业务。
		constructorType := reflect.FuncOf(nil, []reflect.Type{typeOf}, false)
		constructor := reflect.MakeFunc(constructorType, func([]reflect.Value) []reflect.Value { return []reflect.Value{value} }).Interface()
		if err := container.Provide(constructor); err != nil {
			return nil, fmt.Errorf("register configuration %s in construction engine: %w", typeOf, err)
		}
	}
	// Provider 已由 Compiler 排序和校验，Dig 在此只登记调用能力。
	for _, provider := range plan.Providers {
		if err := container.Provide(provider.Constructor.Interface()); err != nil {
			return nil, &diagnostic.ComponentError{Module: provider.Module, Component: provider.Type.String(), Provider: provider.Name, Phase: diagnostic.Construct, Cause: fmt.Errorf("construction engine rejected provider: %w", err)}
		}
	}
	// 别名 Provider 只做类型转换并返回原实例，不会构造第二份 Implementation。
	for _, binding := range plan.Bindings {
		// alias 接收实现并转换成接口，返回的 interface 仍引用同一个底层对象。
		functionType := reflect.FuncOf([]reflect.Type{binding.Implementation}, []reflect.Type{binding.Contract}, false)
		alias := reflect.MakeFunc(functionType, func(values []reflect.Value) []reflect.Value {
			return []reflect.Value{values[0].Convert(binding.Contract)}
		}).Interface()
		if err := container.Provide(alias); err != nil {
			return nil, fmt.Errorf("register contract alias %s: %w", binding.Contract, err)
		}
	}

	// 逐个 Invoke 而不是一次性请求所有对象，确保成功构造后能立即登记并支持精确回滚。
	instances := make([]compiled.Instance, 0, len(plan.Providers))
	// 拓扑顺序逐个请求实例，使返回切片天然也是生命周期正序。
	for _, provider := range plan.Providers {
		// 在每次可能触发用户构造函数前检查取消，停止继续创建新资源。
		if err := ctx.Err(); err != nil {
			return instances, err
		}
		// captured 由临时 consumer 接收 Dig 已解析的准确具体实例。
		var captured reflect.Value
		consumerType := reflect.FuncOf([]reflect.Type{provider.Type}, nil, false)
		consumer := reflect.MakeFunc(consumerType, func(values []reflect.Value) []reflect.Value { captured = values[0]; return nil }).Interface()
		if err := container.Invoke(consumer); err != nil {
			// Dig 错误先归一化，再补充项目模块、组件、Provider 和阶段上下文。
			cause := normalize(err)
			componentErr := &diagnostic.ComponentError{Module: provider.Module, Component: provider.Type.String(), Provider: provider.Name, Phase: diagnostic.Construct, Cause: cause}
			return instances, componentErr
		}
		// 只有 Invoke 完整成功后才登记实例，回滚列表不会包含半构造对象。
		instances = append(instances, compiled.Instance{Provider: provider, Value: captured.Interface()})
	}
	return instances, nil
}

// normalize 把 Dig panic 和解析错误转换为 Kernel 可识别的错误链。
func normalize(err error) error {
	// Dig 错误只在此处解析；公共错误链不会出现 dig.Error 或 dig.PanicError。
	root := dig.RootCause(err)
	// Provider panic 使用项目 PanicError 保存值和 Dig 捕获的堆栈文本。
	var panicError dig.PanicError
	if errors.As(root, &panicError) {
		return &diagnostic.PanicError{Value: panicError.Panic, Stack: []byte(fmt.Sprintf("%+v", panicError))}
	}
	// 其他 Dig 解析错误保留 root 作为原因，但不向上暴露需要理解的控制类型。
	var digError dig.Error
	if errors.As(root, &digError) {
		return fmt.Errorf("construction engine dependency resolution failed: %w", root)
	}
	// 普通 Provider error 已经是业务原因，直接交给 ComponentError 包装。
	return root
}
