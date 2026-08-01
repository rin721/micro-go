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
		constructorType := reflect.FuncOf(nil, []reflect.Type{typeOf}, false)
		constructor := reflect.MakeFunc(constructorType, func([]reflect.Value) []reflect.Value { return []reflect.Value{value} }).Interface()
		if err := container.Provide(constructor); err != nil {
			return nil, fmt.Errorf("register configuration %s in construction engine: %w", typeOf, err)
		}
	}
	for _, provider := range plan.Providers {
		if err := container.Provide(provider.Constructor.Interface()); err != nil {
			return nil, &diagnostic.ComponentError{Module: provider.Module, Component: provider.Type.String(), Provider: provider.Name, Phase: diagnostic.Construct, Cause: fmt.Errorf("construction engine rejected provider: %w", err)}
		}
	}
	// 别名 Provider 只做类型转换并返回原实例，不会构造第二份 Implementation。
	for _, binding := range plan.Bindings {
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
	for _, provider := range plan.Providers {
		if err := ctx.Err(); err != nil {
			return instances, err
		}
		var captured reflect.Value
		consumerType := reflect.FuncOf([]reflect.Type{provider.Type}, nil, false)
		consumer := reflect.MakeFunc(consumerType, func(values []reflect.Value) []reflect.Value { captured = values[0]; return nil }).Interface()
		if err := container.Invoke(consumer); err != nil {
			cause := normalize(err)
			componentErr := &diagnostic.ComponentError{Module: provider.Module, Component: provider.Type.String(), Provider: provider.Name, Phase: diagnostic.Construct, Cause: cause}
			return instances, componentErr
		}
		instances = append(instances, compiled.Instance{Provider: provider, Value: captured.Interface()})
	}
	return instances, nil
}

func normalize(err error) error {
	// Dig 错误只在此处解析；公共错误链不会出现 dig.Error 或 dig.PanicError。
	root := dig.RootCause(err)
	var panicError dig.PanicError
	if errors.As(root, &panicError) {
		return &diagnostic.PanicError{Value: panicError.Panic, Stack: []byte(fmt.Sprintf("%+v", panicError))}
	}
	var digError dig.Error
	if errors.As(root, &digError) {
		return fmt.Errorf("construction engine dependency resolution failed: %w", root)
	}
	return root
}
