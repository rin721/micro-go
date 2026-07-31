// Package source 提供 internal/kernel/config.Source 的标准配置来源实现。
//
// Kernel 只定义读取协议，本包负责接触文件系统、环境变量和 flag.FlagSet。这样新增
// 远程配置或替换来源实现时，不需要修改 Kernel 的快照与 Reload 语义。
package source

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/rin721/micro-go/internal/kernel/config"
)

type source struct {
	name  string
	load  func(context.Context) (config.Payload, error)
	watch *config.WatchDescriptor
}

func (s source) Name() string { return s.name }

func (s source) Load(ctx context.Context) (config.Payload, error) {
	// 即使具体来源只是内存 map，也统一尊重取消，使所有 Source 具有相同边界语义。
	if err := ctx.Err(); err != nil {
		return config.Payload{}, err
	}
	return s.load(ctx)
}

type watchSource struct{ source }

func (s watchSource) WatchDescriptor() config.WatchDescriptor { return *s.watch }

// FromValues 创建内存配置源；输入会在每次 Load 时深复制。
func FromValues(values map[string]any) config.Source {
	return source{name: "values", load: func(context.Context) (config.Payload, error) {
		cloned, err := cloneMap(values)
		return config.Payload{Values: cloned, Format: config.FormatMap}, err
	}}
}

// FromFile 创建 YAML 或 JSON 文件配置源。
func FromFile(path string) config.Source {
	abs, _ := filepath.Abs(path)
	format := config.FormatYAML
	if strings.EqualFold(filepath.Ext(path), ".json") {
		format = config.FormatJSON
	}
	descriptor := config.WatchDescriptor{Path: abs}
	return watchSource{source{name: "file:" + abs, watch: &descriptor, load: func(ctx context.Context) (config.Payload, error) {
		if err := ctx.Err(); err != nil {
			return config.Payload{}, err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return config.Payload{}, fmt.Errorf("read config file %q: %w", abs, err)
		}
		return config.Payload{Bytes: data, Format: format}, nil
	}}}
}

// FromEnvironment 使用 PREFIX_ 过滤环境变量，双下划线表示配置层级。
func FromEnvironment(prefix string) config.Source {
	prefix = strings.TrimSuffix(strings.ToUpper(prefix), "_") + "_"
	return source{name: "environment:" + strings.TrimSuffix(prefix, "_"), load: func(context.Context) (config.Payload, error) {
		values := make(map[string]any)
		for _, entry := range os.Environ() {
			key, value, ok := strings.Cut(entry, "=")
			if !ok || !strings.HasPrefix(strings.ToUpper(key), prefix) {
				continue
			}
			path := strings.ToLower(strings.TrimPrefix(strings.ToUpper(key), prefix))
			values[strings.ReplaceAll(path, "__", ".")] = value
		}
		return config.Payload{Values: values, Format: config.FormatMap}, nil
	}}
}

// FromFlags 读取已经解析的标准库 FlagSet，Flag 名称直接作为点分配置路径。
func FromFlags(flags *flag.FlagSet) config.Source {
	return source{name: "flags", load: func(context.Context) (config.Payload, error) {
		if flags == nil {
			return config.Payload{}, errors.New("config flag set is nil")
		}
		values := make(map[string]any)
		flags.Visit(func(item *flag.Flag) {
			if getter, ok := item.Value.(flag.Getter); ok {
				values[item.Name] = getter.Get()
				return
			}
			values[item.Name] = item.Value.String()
		})
		return config.Payload{Values: values, Format: config.FormatMap}, nil
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
	// 不使用 JSON 往返复制，避免在进入配置引擎前改变整数等动态值的实际类型。
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

var _ config.WatchSource = watchSource{}
