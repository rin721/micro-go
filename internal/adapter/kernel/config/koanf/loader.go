// Package koanfadapter 使用 Koanf 完成来源合并和强类型解码，并把 validator 错误
// 归一化为项目 ValidationIssue。任何第三方类型都不会离开该包。
package koanfadapter

import (
	"context"
	"crypto/sha256"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	playground "github.com/go-playground/validator/v10"
	"github.com/go-viper/mapstructure/v2"
	jsonparser "github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiled"
	public "github.com/rin721/micro-go/internal/kernel/config"
)

// Loader 封装 Koanf 配置流水线依赖。clock 是内部可替换点，用于让 Snapshot 时间可测试。
type Loader struct {
	// validator 执行第三方标签校验，但错误会在本包转换为项目模型。
	validator *playground.Validate
	// clock 生成快照时间；测试可替换它而无需修改系统时钟。
	clock func() time.Time
}

// New 创建启用 required struct 语义的 Loader。
func New() *Loader {
	// required struct 选项让嵌套值结构也遵守 required 语义，避免零值悄然通过。
	return &Loader{validator: playground.New(playground.WithRequiredStructEnabled()), clock: time.Now}
}

// Load 从空 Koanf 实例开始按顺序加载全部 Source，再解码、验证并生成不可变 Snapshot。
// 每次都从空实例开始是候选配置事务性的基础：失败候选不会修改当前快照。
func (l *Loader) Load(ctx context.Context, version uint64, sources []public.Source, declarations []compiled.Config) (public.Loaded, error) {
	// StrictMerge 让 map 与标量等类型冲突直接失败，而不是静默覆盖成不可预测结构。
	k := koanf.NewWithConf(koanf.Conf{Delim: ".", StrictMerge: true})
	// 调用顺序就是覆盖优先级，后加载来源覆盖前者。
	for _, source := range sources {
		// nil 来源无法提供名称或读取行为，作为调用方装配错误立即拒绝。
		if source == nil {
			return public.Loaded{}, errors.New("configuration source is nil")
		}
		payload, err := source.Load(ctx)
		if err != nil {
			return public.Loaded{}, fmt.Errorf("load configuration source %q: %w", source.Name(), err)
		}
		// Format 是项目契约；这里选择对应 Koanf Provider/Parser，并将第三方类型留在包内。
		switch payload.Format {
		case public.FormatMap:
			err = k.Load(confmap.Provider(payload.Values, "."), nil)
		case public.FormatJSON:
			err = k.Load(rawbytes.Provider(payload.Bytes), jsonparser.Parser())
		case public.FormatYAML:
			err = k.Load(rawbytes.Provider(payload.Bytes), yaml.Parser())
		default:
			err = fmt.Errorf("unsupported configuration format %q", payload.Format)
		}
		// 每个来源合并失败都补充来源名称，同时保留 Koanf 原始原因链。
		if err != nil {
			return public.Loaded{}, fmt.Errorf("merge configuration source %q: %w", source.Name(), err)
		}
	}

	// 按类型名排序使 Snapshot 条目和失败顺序不受 map 遍历影响。
	ordered := append([]compiled.Config(nil), declarations...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Type.String() < ordered[j].Type.String() })
	if err := validateDeclaredPaths(k.Keys(), ordered); err != nil {
		return public.Loaded{}, err
	}
	// Values 服务于本次构造注入；entries 服务于发布后的不可变 Reload 快照。
	values := make(map[reflect.Type]reflect.Value, len(ordered))
	entries := make([]public.SnapshotEntry, 0, len(ordered))
	// 每份声明独立创建指针目标，避免不同配置类型共享反射存储。
	for _, declaration := range ordered {
		pointer := reflect.New(declaration.Type)
		// Decoder 只开放项目需要的 duration 和 TextUnmarshaler 转换，并拒绝未消费字段。
		decoderConfig := &mapstructure.DecoderConfig{
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeDurationHookFunc(),
				mapstructure.TextUnmarshallerHookFunc(),
			),
			ErrorUnused:      true,
			WeaklyTypedInput: true,
		}
		// 点分 Path 将合并树的所有权边界映射到当前强类型值。
		if err := k.UnmarshalWithConf(declaration.Path, pointer.Interface(), koanf.UnmarshalConf{Tag: "yaml", DecoderConfig: decoderConfig}); err != nil {
			return public.Loaded{}, fmt.Errorf("decode configuration %s at %q: %w", declaration.Type, declaration.Path, err)
		}
		// 标签与领域校验必须在值进入构造注入或 Snapshot 前全部成功。
		if err := l.validate(pointer.Interface()); err != nil {
			return public.Loaded{}, fmt.Errorf("validate configuration %s at %q: %w", declaration.Type, declaration.Path, err)
		}
		// Elem 取得非指针配置值，使其类型与 Compiler 声明准确一致。
		value := pointer.Elem()
		// 规范化 JSON 同时承担深复制载体和内容摘要输入，Snapshot 不保留可变 struct 引用。
		data, err := stdjson.Marshal(value.Interface())
		if err != nil {
			return public.Loaded{}, fmt.Errorf("snapshot configuration %s: %w", declaration.Type, err)
		}
		values[declaration.Type] = value
		entries = append(entries, public.SnapshotEntry{Type: declaration.Type, Data: data, Hash: sha256.Sum256(data)})
	}
	// 所有声明成功后才分配一次 UTC 时间并整体发布 Loaded，失败路径不会产生候选快照。
	loadedAt := l.clock().UTC()
	return public.Loaded{Snapshot: public.NewSnapshot(version, loadedAt, entries), Values: values}, nil
}

// validateDeclaredPaths 拒绝没有模块声明所有权的配置键，避免拼写错误被静默忽略。
func validateDeclaredPaths(keys []string, declarations []compiled.Config) error {
	// Koanf 返回叶子键；每个键必须等于某个声明路径或位于其子树中。
	for _, key := range keys {
		owned := false
		// 声明数量通常很小，直接遍历可保持规则清晰且不引入第二套路径索引。
		for _, declaration := range declarations {
			path := strings.TrimSpace(declaration.Path)
			if key == path || strings.HasPrefix(key, path+".") {
				owned = true
				break
			}
		}
		// 第一个无主键立即返回，排序后的 Koanf Keys 让失败顺序保持稳定。
		if !owned {
			return fmt.Errorf("configuration key %q has no owning module", key)
		}
	}
	return nil
}

// validate 汇总第三方标签校验和项目自有领域校验，并保留全部原因链。
func (l *Loader) validate(value any) error {
	// 标签校验与项目 Validate 契约统一收集，调用方只需理解 ValidationIssue。
	var issues []public.ValidationIssue
	var causes []error
	if err := l.validator.Struct(value); err != nil {
		// validator 也可能因调用方式错误返回非字段集合错误，此时不能伪造成配置问题。
		var validationErrors playground.ValidationErrors
		if !errors.As(err, &validationErrors) {
			return fmt.Errorf("invalid validation input: %w", err)
		}
		// 原始聚合错误只加入一次，供 errors.As 继续识别第三方字段细节。
		causes = append(causes, err)
		// 每个字段问题转换成不泄漏第三方类型的稳定项目值。
		for _, item := range validationErrors {
			issues = append(issues, public.ValidationIssue{Path: item.Namespace(), Rule: item.Tag(), Message: "validation rule " + item.Tag() + " failed"})
		}
	}
	// 自定义 Validate 用于表达跨字段或领域规则，用户配置不需要导入 validator 包。
	if validator, ok := value.(public.Validator); ok {
		// 领域校验失败与标签问题共同汇总，调用方可以一次看到所有已知原因。
		if err := validator.Validate(); err != nil {
			causes = append(causes, err)
			issues = append(issues, public.ValidationIssue{Path: "", Rule: "Validate", Message: err.Error()})
		}
	}
	// 只有确实收集到问题时才返回 ValidationError；空集合代表配置可发布。
	if len(issues) > 0 {
		return public.NewValidationError(issues, causes...)
	}
	return nil
}
