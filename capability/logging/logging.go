// Package logging 定义实现无关的结构化日志能力。
package logging

import (
	"context"
	"time"
)

type Field struct {
	Key   string
	Value any
}

func String(key, value string) Field                 { return Field{Key: key, Value: value} }
func Int(key string, value int) Field                { return Field{Key: key, Value: value} }
func Bool(key string, value bool) Field              { return Field{Key: key, Value: value} }
func Duration(key string, value time.Duration) Field { return Field{Key: key, Value: value} }
func Time(key string, value time.Time) Field         { return Field{Key: key, Value: value} }
func Error(err error) Field                          { return Field{Key: "error", Value: err} }

type Logger interface {
	Debug(context.Context, string, ...Field)
	Info(context.Context, string, ...Field)
	Warn(context.Context, string, ...Field)
	Error(context.Context, string, ...Field)
	With(...Field) Logger
	Named(string) Logger
}

type Noop struct{}

func (Noop) Debug(context.Context, string, ...Field) {}
func (Noop) Info(context.Context, string, ...Field)  {}
func (Noop) Warn(context.Context, string, ...Field)  {}
func (Noop) Error(context.Context, string, ...Field) {}
func (Noop) With(...Field) Logger                    { return Noop{} }
func (Noop) Named(string) Logger                     { return Noop{} }
