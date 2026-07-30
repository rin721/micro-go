package digadapter

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/rin721/micro-go/internal/di/compiled"
	"github.com/rin721/micro-go/kernel/diagnostic"
	"github.com/rin721/micro-go/kernel/lifecycle"
	"go.uber.org/dig"
)

type Engine struct{}

func New() *Engine { return &Engine{} }

func (*Engine) Construct(ctx context.Context, plan *compiled.Plan, configs map[reflect.Type]reflect.Value) ([]compiled.Instance, error) {
	container := dig.New(dig.DeferAcyclicVerification(), dig.RecoverFromPanics())
	for typeOf, value := range configs {
		constructorType := reflect.FuncOf(nil, []reflect.Type{typeOf}, false)
		constructor := reflect.MakeFunc(constructorType, func([]reflect.Value) []reflect.Value { return []reflect.Value{value} }).Interface()
		if err := container.Provide(constructor); err != nil {
			return nil, fmt.Errorf("register configuration %s in construction engine", typeOf)
		}
	}
	for _, provider := range plan.Providers {
		if err := container.Provide(provider.Constructor.Interface()); err != nil {
			return nil, &diagnostic.ComponentError{Module: provider.Module, Component: provider.Type.String(), Provider: provider.Name, Phase: diagnostic.Construct, Cause: errors.New("construction engine rejected provider")}
		}
	}
	for _, binding := range plan.Bindings {
		functionType := reflect.FuncOf([]reflect.Type{binding.Implementation}, []reflect.Type{binding.Contract}, false)
		alias := reflect.MakeFunc(functionType, func(values []reflect.Value) []reflect.Value {
			return []reflect.Value{values[0].Convert(binding.Contract)}
		}).Interface()
		if err := container.Provide(alias); err != nil {
			return nil, fmt.Errorf("register contract alias %s", binding.Contract)
		}
	}

	instances := make([]compiled.Instance, 0, len(plan.Providers))
	for _, provider := range plan.Providers {
		if err := ctx.Err(); err != nil {
			return nil, rollback(ctx, instances, err)
		}
		var captured reflect.Value
		consumerType := reflect.FuncOf([]reflect.Type{provider.Type}, nil, false)
		consumer := reflect.MakeFunc(consumerType, func(values []reflect.Value) []reflect.Value { captured = values[0]; return nil }).Interface()
		if err := container.Invoke(consumer); err != nil {
			cause := normalize(err)
			componentErr := &diagnostic.ComponentError{Module: provider.Module, Component: provider.Type.String(), Provider: provider.Name, Phase: diagnostic.Construct, Cause: cause}
			return nil, rollback(ctx, instances, componentErr)
		}
		instances = append(instances, compiled.Instance{Provider: provider, Value: captured.Interface()})
	}
	return instances, nil
}

func normalize(err error) error {
	root := dig.RootCause(err)
	var panicError dig.PanicError
	if errors.As(root, &panicError) {
		return &diagnostic.PanicError{Value: panicError.Panic, Stack: []byte(fmt.Sprintf("%+v", panicError))}
	}
	var digError dig.Error
	if errors.As(root, &digError) {
		return errors.New("construction engine dependency resolution failed")
	}
	return root
}

func rollback(ctx context.Context, instances []compiled.Instance, cause error) error {
	errorsList := []error{cause}
	for index := len(instances) - 1; index >= 0; index-- {
		closer, ok := instances[index].Value.(lifecycle.Closer)
		if !ok {
			continue
		}
		if err := callClose(ctx, closer); err != nil {
			provider := instances[index].Provider
			errorsList = append(errorsList, &diagnostic.ComponentError{Module: provider.Module, Component: provider.Type.String(), Provider: provider.Name, Phase: diagnostic.Close, Cause: err})
		}
	}
	return errors.Join(errorsList...)
}

func callClose(ctx context.Context, closer lifecycle.Closer) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = diagnostic.NewPanicError(value)
		}
	}()
	return closer.Close(ctx)
}
