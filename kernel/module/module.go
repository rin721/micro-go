// Package module 定义模块注册公共契约和泛型声明辅助函数。
package module

import (
	"fmt"
	"reflect"
)

type Module interface {
	Name() string
	Register(Registry) error
}

type ProviderDeclaration struct {
	Constructor any
}

type BindingDeclaration struct {
	Contract       reflect.Type
	Implementation reflect.Type
}

type ExportDeclaration struct {
	Contract reflect.Type
}

type ConfigDeclaration struct {
	Type reflect.Type
	Path string
}

type Registry interface {
	RegisterProvider(ProviderDeclaration) error
	RegisterBinding(BindingDeclaration) error
	RegisterExport(ExportDeclaration) error
	RegisterConfig(ConfigDeclaration) error
}

func Provide(registry Registry, constructor any) error {
	if registry == nil {
		return fmt.Errorf("module registry is nil")
	}
	return registry.RegisterProvider(ProviderDeclaration{Constructor: constructor})
}

func Bind[Contract, Implementation any](registry Registry) error {
	if registry == nil {
		return fmt.Errorf("module registry is nil")
	}
	contract := reflect.TypeOf((*Contract)(nil)).Elem()
	implementation := reflect.TypeOf((*Implementation)(nil)).Elem()
	return registry.RegisterBinding(BindingDeclaration{Contract: contract, Implementation: implementation})
}

func Export[Contract any](registry Registry) error {
	if registry == nil {
		return fmt.Errorf("module registry is nil")
	}
	contract := reflect.TypeOf((*Contract)(nil)).Elem()
	return registry.RegisterExport(ExportDeclaration{Contract: contract})
}

func Config[T any](registry Registry, path string) error {
	if registry == nil {
		return fmt.Errorf("module registry is nil")
	}
	typeOf := reflect.TypeOf((*T)(nil)).Elem()
	return registry.RegisterConfig(ConfigDeclaration{Type: typeOf, Path: path})
}
