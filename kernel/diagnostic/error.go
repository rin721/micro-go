// Package diagnostic 定义框架各层共享的稳定错误模型。
package diagnostic

import (
	"fmt"
	"runtime/debug"
)

type Phase string

const (
	ModuleRegister Phase = "ModuleRegister"
	ConfigLoad     Phase = "ConfigLoad"
	ConfigDecode   Phase = "ConfigDecode"
	ConfigValidate Phase = "ConfigValidate"
	GraphCompile   Phase = "GraphCompile"
	Construct      Phase = "Construct"
	Prepare        Phase = "Prepare"
	Start          Phase = "Start"
	Run            Phase = "Run"
	Stop           Phase = "Stop"
	Close          Phase = "Close"
	Observe        Phase = "Observe"
)

type ComponentError struct {
	Module    string
	Component string
	Provider  string
	Phase     Phase
	Cause     error
}

func (e *ComponentError) Error() string {
	return fmt.Sprintf("phase=%s module=%s component=%s provider=%s: %v", e.Phase, e.Module, e.Component, e.Provider, e.Cause)
}

func (e *ComponentError) Unwrap() error { return e.Cause }

type PanicError struct {
	Value any
	Stack []byte
}

func NewPanicError(value any) *PanicError {
	return &PanicError{Value: value, Stack: debug.Stack()}
}

func (e *PanicError) Error() string { return fmt.Sprintf("panic: %v", e.Value) }
