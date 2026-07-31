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
	"time"

	playground "github.com/go-playground/validator/v10"
	jsonparser "github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	public "github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/pkg/adapter/kernel/di/compiled"
)

// Loader 封装 Koanf 配置流水线依赖。clock 是内部可替换点，用于让 Snapshot 时间可测试。
type Loader struct {
	validator *playground.Validate
	clock     func() time.Time
}

// New 创建启用 required struct 语义的 Loader。
func New() *Loader {
	return &Loader{validator: playground.New(playground.WithRequiredStructEnabled()), clock: time.Now}
}

// Load 从空 Koanf 实例开始按顺序加载全部 Source，再解码、验证并生成不可变 Snapshot。
// 每次都从空实例开始是候选配置事务性的基础：失败候选不会修改当前快照。
func (l *Loader) Load(ctx context.Context, version uint64, sources []public.Source, declarations []compiled.Config) (public.Loaded, error) {
	// StrictMerge 让 map 与标量等类型冲突直接失败，而不是静默覆盖成不可预测结构。
	k := koanf.NewWithConf(koanf.Conf{Delim: ".", StrictMerge: true})
	// 调用顺序就是覆盖优先级，后加载来源覆盖前者。
	for _, source := range sources {
		if source == nil {
			return public.Loaded{}, errors.New("configuration source is nil")
		}
		payload, err := source.Load(ctx)
		if err != nil {
			return public.Loaded{}, fmt.Errorf("load configuration source %q: %w", source.Name(), err)
		}
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
		if err != nil {
			return public.Loaded{}, fmt.Errorf("merge configuration source %q: %s", source.Name(), err)
		}
	}

	// 按类型名排序使 Snapshot 条目和失败顺序不受 map 遍历影响。
	ordered := append([]compiled.Config(nil), declarations...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Type.String() < ordered[j].Type.String() })
	values := make(map[reflect.Type]reflect.Value, len(ordered))
	entries := make([]public.SnapshotEntry, 0, len(ordered))
	for _, declaration := range ordered {
		pointer := reflect.New(declaration.Type)
		if err := k.UnmarshalWithConf(declaration.Path, pointer.Interface(), koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
			return public.Loaded{}, fmt.Errorf("decode configuration %s at %q: %s", declaration.Type, declaration.Path, err)
		}
		if err := l.validate(pointer.Interface()); err != nil {
			return public.Loaded{}, fmt.Errorf("validate configuration %s at %q: %w", declaration.Type, declaration.Path, err)
		}
		value := pointer.Elem()
		// 规范化 JSON 同时承担深复制载体和内容摘要输入，Snapshot 不保留可变 struct 引用。
		data, err := stdjson.Marshal(value.Interface())
		if err != nil {
			return public.Loaded{}, fmt.Errorf("snapshot configuration %s: %w", declaration.Type, err)
		}
		values[declaration.Type] = value
		entries = append(entries, public.SnapshotEntry{Type: declaration.Type, Data: data, Hash: sha256.Sum256(data)})
	}
	loadedAt := l.clock().UTC()
	return public.Loaded{Snapshot: public.NewSnapshot(version, loadedAt, entries), Values: values}, nil
}

func (l *Loader) validate(value any) error {
	// 标签校验与项目 Validate 契约统一收集，调用方只需理解 ValidationIssue。
	var issues []public.ValidationIssue
	if err := l.validator.Struct(value); err != nil {
		var validationErrors playground.ValidationErrors
		if !errors.As(err, &validationErrors) {
			return fmt.Errorf("invalid validation input")
		}
		for _, item := range validationErrors {
			issues = append(issues, public.ValidationIssue{Path: item.Namespace(), Rule: item.Tag(), Message: "validation rule " + item.Tag() + " failed"})
		}
	}
	// 自定义 Validate 用于表达跨字段或领域规则，用户配置不需要导入 validator 包。
	if validator, ok := value.(public.Validator); ok {
		if err := validator.Validate(); err != nil {
			issues = append(issues, public.ValidationIssue{Path: "", Rule: "Validate", Message: err.Error()})
		}
	}
	if len(issues) > 0 {
		return &public.ValidationError{Issues: issues}
	}
	return nil
}
