// Package config 定义项目拥有的配置源、快照和校验契约。
package config

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

// Format 描述原始配置数据格式。解析器实现属于内部适配层。
type Format string

const (
	FormatMap  Format = "map"
	FormatJSON Format = "json"
	FormatYAML Format = "yaml"
)

// Payload 是配置源与配置引擎之间的项目自有传输模型。
type Payload struct {
	Values map[string]any
	Bytes  []byte
	Format Format
}

// Source 只负责读取一份配置事实，不负责合并或强类型解码。
type Source interface {
	Name() string
	Load(context.Context) (Payload, error)
}

// WatchDescriptor 描述由内部监听适配器观察的文件。
type WatchDescriptor struct {
	Path string
}

// Change 是监听适配器向运行时报告的项目自有变更事件。
type Change struct {
	Source string
	At     time.Time
}

// WatchSource 是可以声明监听目标的配置源。
type WatchSource interface {
	Source
	WatchDescriptor() WatchDescriptor
}

type source struct {
	name  string
	load  func(context.Context) (Payload, error)
	watch *WatchDescriptor
}

func (s source) Name() string { return s.name }

func (s source) Load(ctx context.Context) (Payload, error) {
	if err := ctx.Err(); err != nil {
		return Payload{}, err
	}
	return s.load(ctx)
}

type watchSource struct{ source }

func (s watchSource) WatchDescriptor() WatchDescriptor { return *s.watch }

// FromValues 创建内存配置源。输入会在每次 Load 时深复制。
func FromValues(values map[string]any) Source {
	return source{name: "values", load: func(context.Context) (Payload, error) {
		cloned, err := cloneMap(values)
		return Payload{Values: cloned, Format: FormatMap}, err
	}}
}

// FromFile 创建 YAML 或 JSON 文件配置源。
func FromFile(path string) Source {
	abs, _ := filepath.Abs(path)
	format := FormatYAML
	if strings.EqualFold(filepath.Ext(path), ".json") {
		format = FormatJSON
	}
	descriptor := WatchDescriptor{Path: abs}
	return watchSource{source{name: "file:" + abs, watch: &descriptor, load: func(ctx context.Context) (Payload, error) {
		if err := ctx.Err(); err != nil {
			return Payload{}, err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return Payload{}, fmt.Errorf("read config file %q: %w", abs, err)
		}
		return Payload{Bytes: data, Format: format}, nil
	}}}
}

// FromEnvironment 使用 PREFIX_ 过滤环境变量，双下划线表示配置层级。
func FromEnvironment(prefix string) Source {
	prefix = strings.TrimSuffix(strings.ToUpper(prefix), "_") + "_"
	return source{name: "environment:" + strings.TrimSuffix(prefix, "_"), load: func(context.Context) (Payload, error) {
		values := make(map[string]any)
		for _, entry := range os.Environ() {
			key, value, ok := strings.Cut(entry, "=")
			if !ok || !strings.HasPrefix(strings.ToUpper(key), prefix) {
				continue
			}
			path := strings.ToLower(strings.TrimPrefix(strings.ToUpper(key), prefix))
			path = strings.ReplaceAll(path, "__", ".")
			values[path] = value
		}
		return Payload{Values: values, Format: FormatMap}, nil
	}}
}

// FromFlags 读取已经解析的标准库 FlagSet，Flag 名称直接作为点分配置路径。
func FromFlags(flags *flag.FlagSet) Source {
	return source{name: "flags", load: func(context.Context) (Payload, error) {
		if flags == nil {
			return Payload{}, errors.New("config flag set is nil")
		}
		values := make(map[string]any)
		flags.Visit(func(item *flag.Flag) {
			if getter, ok := item.Value.(flag.Getter); ok {
				values[item.Name] = getter.Get()
				return
			}
			values[item.Name] = item.Value.String()
		})
		return Payload{Values: values, Format: FormatMap}, nil
	}}
}

func cloneMap(values map[string]any) (map[string]any, error) {
	if values == nil {
		return map[string]any{}, nil
	}
	cloned, err := cloneReflect(reflect.ValueOf(values))
	if err != nil {
		return nil, fmt.Errorf("clone config values: %w", err)
	}
	return cloned.Interface().(map[string]any), nil
}

func cloneReflect(value reflect.Value) (reflect.Value, error) {
	if !value.IsValid() {
		return value, nil
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		cloned, err := cloneReflect(value.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		result := reflect.New(value.Type()).Elem()
		result.Set(cloned)
		return result, nil
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		cloned, err := cloneReflect(value.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(cloned)
		return result, nil
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			cloned, err := cloneReflect(iterator.Value())
			if err != nil {
				return reflect.Value{}, err
			}
			result.SetMapIndex(iterator.Key(), cloned)
		}
		return result, nil
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := range value.Len() {
			cloned, err := cloneReflect(value.Index(index))
			if err != nil {
				return reflect.Value{}, err
			}
			result.Index(index).Set(cloned)
		}
		return result, nil
	case reflect.Array:
		result := reflect.New(value.Type()).Elem()
		for index := range value.Len() {
			cloned, err := cloneReflect(value.Index(index))
			if err != nil {
				return reflect.Value{}, err
			}
			result.Index(index).Set(cloned)
		}
		return result, nil
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return reflect.Value{}, fmt.Errorf("unsupported mutable value kind %s", value.Kind())
	default:
		return value, nil
	}
}
