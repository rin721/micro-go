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

// source 用闭包统一内存、文件、环境和 Flag 来源，同时只暴露 Kernel Source 契约。
type source struct {
	// name 是事件和错误中可观察的稳定来源标识。
	name string
	// load 封装具体 I/O 或复制逻辑，外层方法统一处理 Context。
	load func(context.Context) (config.Payload, error)
	// watch 仅对文件来源存在，普通 Source 保持 nil。
	watch *config.WatchDescriptor
}

// Name 返回创建来源时确定的稳定名称。
func (s source) Name() string { return s.name }

// Load 在调用具体来源前统一检查取消信号。
func (s source) Load(ctx context.Context) (config.Payload, error) {
	// 即使具体来源只是内存 map，也统一尊重取消，使所有 Source 具有相同边界语义。
	if err := ctx.Err(); err != nil {
		return config.Payload{}, err
	}
	// Context 有效时把读取职责交给来源闭包，并完整返回其结果。
	return s.load(ctx)
}

// watchSource 在普通 source 上增加不可变文件监听描述。
type watchSource struct {
	// source 提供名称和读取行为。
	source
}

// WatchDescriptor 返回描述值副本，调用方不能替换内部指针。
func (s watchSource) WatchDescriptor() config.WatchDescriptor { return *s.watch }

// FromValues 创建内存配置源；输入会在每次 Load 时深复制。
func FromValues(values map[string]any) config.Source {
	// 每次 Load 都重新深复制输入，调用方后续修改原 map 不会污染已返回 Payload。
	return source{name: "values", load: func(context.Context) (config.Payload, error) {
		cloned, err := cloneMap(values)
		return config.Payload{Values: cloned, Format: config.FormatMap}, err
	}}
}

// FromFile 创建 YAML 或 JSON 文件配置源。
// 路径在 Source 进入 Runtime 前完成校验，避免 WatchDescriptor 携带未解析路径。
func FromFile(path string) (config.Source, error) {
	// 空白路径没有明确文件所有权，必须在触碰文件系统前拒绝。
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("configuration file path is empty")
	}
	// 绝对路径同时用于读取、监听和 Source 名称，消除工作目录变化带来的歧义。
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve configuration file path %q: %w", path, err)
	}
	// 非 JSON 扩展名按 YAML 处理，与项目默认配置格式保持一致。
	format := config.FormatYAML
	if strings.EqualFold(filepath.Ext(path), ".json") {
		format = config.FormatJSON
	}
	// descriptor 与闭包共享同一绝对路径，但通过值返回避免外部修改。
	descriptor := config.WatchDescriptor{Path: abs}
	return watchSource{source{name: "file:" + abs, watch: &descriptor, load: func(ctx context.Context) (config.Payload, error) {
		// 文件读取前再次检查取消，覆盖 Source 创建后到 Load 调用之间的取消窗口。
		if err := ctx.Err(); err != nil {
			return config.Payload{}, err
		}
		// 一次性读取完整文件，让 Loader 对单份一致字节执行解析和合并。
		data, err := os.ReadFile(abs)
		if err != nil {
			return config.Payload{}, fmt.Errorf("read config file %q: %w", abs, err)
		}
		// 原始字节和已确定格式交给 Loader，Source 不承担语法解析职责。
		return config.Payload{Bytes: data, Format: format}, nil
	}}}, nil
}

// FromEnvironment 使用 PREFIX_ 过滤环境变量，双下划线表示配置层级。
// excludedKeys 使用完整环境变量名，适合排除选择配置文件等进程控制变量。
func FromEnvironment(prefix string, excludedKeys ...string) config.Source {
	// 统一为大写且恰好一个尾随下划线，使 Windows/Linux 匹配规则一致。
	prefix = strings.TrimSuffix(strings.ToUpper(prefix), "_") + "_"
	// 排除集合使用完整规范化变量名，避免进程控制变量进入配置树。
	excluded := make(map[string]struct{}, len(excludedKeys))
	for _, key := range excludedKeys {
		if normalized := strings.ToUpper(strings.TrimSpace(key)); normalized != "" {
			excluded[normalized] = struct{}{}
		}
	}
	return source{name: "environment:" + strings.TrimSuffix(prefix, "_"), load: func(context.Context) (config.Payload, error) {
		// 扁平点分键由 Koanf confmap Provider 转换为嵌套树。
		values := make(map[string]any)
		// os.Environ 提供当前进程快照；每次 Load 都重新读取以支持候选重建。
		for _, entry := range os.Environ() {
			key, value, ok := strings.Cut(entry, "=")
			normalizedKey := strings.ToUpper(key)
			if !ok || !strings.HasPrefix(normalizedKey, prefix) {
				continue
			}
			// 明确排除的键即使拥有前缀也不能成为模块配置。
			if _, skip := excluded[normalizedKey]; skip {
				continue
			}
			// 去掉前缀后转小写，双下划线映射为层级分隔点。
			path := strings.ToLower(strings.TrimPrefix(normalizedKey, prefix))
			values[strings.ReplaceAll(path, "__", ".")] = value
		}
		return config.Payload{Values: values, Format: config.FormatMap}, nil
	}}
}

// FromFlags 读取已经解析的标准库 FlagSet，Flag 名称直接作为点分配置路径。
func FromFlags(flags *flag.FlagSet) config.Source {
	return source{name: "flags", load: func(context.Context) (config.Payload, error) {
		// nil FlagSet 是装配错误，不能退回全局 command line 集合。
		if flags == nil {
			return config.Payload{}, errors.New("config flag set is nil")
		}
		// Visit 只遍历用户显式设置的 Flag，未设置项不覆盖低优先级来源。
		values := make(map[string]any)
		flags.Visit(func(item *flag.Flag) {
			// Getter 可保留 int、bool、duration 等动态类型，避免全部降级为字符串。
			if getter, ok := item.Value.(flag.Getter); ok {
				values[item.Name] = getter.Get()
				return
			}
			// 不支持 Getter 的自定义 Flag 只能使用其稳定文本表示。
			values[item.Name] = item.Value.String()
		})
		return config.Payload{Values: values, Format: config.FormatMap}, nil
	}}
}

// cloneMap 深复制调用方提供的 map，并保持其中动态值的 Go 类型。
func cloneMap(values map[string]any) (map[string]any, error) {
	// nil 输入按空配置源处理，避免向 Koanf 传递含糊的 nil map。
	if values == nil {
		return map[string]any{}, nil
	}
	// 反射复制覆盖嵌套 map、slice、array、pointer 和 interface。
	cloned, err := cloneReflect(reflect.ValueOf(values))
	if err != nil {
		return nil, fmt.Errorf("clone config values: %w", err)
	}
	// 根输入已由签名保证是 map[string]any，因此复制结果可以安全断言同型。
	return cloned.Interface().(map[string]any), nil
}

// cloneReflect 递归复制可变复合值，并拒绝无法安全复制的运行期资源类型。
func cloneReflect(value reflect.Value) (reflect.Value, error) {
	// 不使用 JSON 往返复制，避免在进入配置引擎前改变整数等动态值的实际类型。
	if !value.IsValid() {
		return value, nil
	}
	// 每种 Kind 明确定义 nil、分配和元素递归语义，禁止隐式共享可变底层存储。
	switch value.Kind() {
	case reflect.Interface:
		// nil interface 保持准确接口类型的零值。
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		// 复制动态元素后重新装入原接口类型。
		cloned, err := cloneReflect(value.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		result := reflect.New(value.Type()).Elem()
		result.Set(cloned)
		return result, nil
	case reflect.Pointer:
		// nil 指针保持 nil，不创建带零值元素的新指针。
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		// 非 nil 指针获得独立分配，并递归复制其目标。
		cloned, err := cloneReflect(value.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(cloned)
		return result, nil
	case reflect.Map:
		// nil map 与空 map 语义不同，因此保留 nil。
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		// map key 视为不可变标识直接复用，value 递归复制以切断可变引用。
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
		// nil slice 保持 nil，非 nil 空 slice 会在下方创建独立空底层数组描述。
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		// 长度和容量都固定为原长度，避免无意保留调用方的额外容量。
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
		// 数组没有 nil 状态，为整个值创建新存储并逐项复制。
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
		// 这些 Kind 代表行为、通信或裸地址，复制引用会破坏资源所有权边界。
		return reflect.Value{}, fmt.Errorf("unsupported mutable value kind %s", value.Kind())
	default:
		// 标量、字符串和其他不可变值可安全按值返回。
		return value, nil
	}
}

// 编译期断言确保文件来源同时满足读取和监听契约。
var _ config.WatchSource = watchSource{}
