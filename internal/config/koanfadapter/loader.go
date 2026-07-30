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
	"github.com/rin721/micro-go/internal/config/loading"
	"github.com/rin721/micro-go/internal/di/compiled"
	public "github.com/rin721/micro-go/kernel/config"
)

type Loader struct {
	validator *playground.Validate
	clock     func() time.Time
}

func New() *Loader {
	return &Loader{validator: playground.New(playground.WithRequiredStructEnabled()), clock: time.Now}
}

func (l *Loader) Load(ctx context.Context, version uint64, sources []public.Source, declarations []compiled.Config) (loading.Loaded, error) {
	k := koanf.NewWithConf(koanf.Conf{Delim: ".", StrictMerge: true})
	for _, source := range sources {
		if source == nil {
			return loading.Loaded{}, errors.New("configuration source is nil")
		}
		payload, err := source.Load(ctx)
		if err != nil {
			return loading.Loaded{}, fmt.Errorf("load configuration source %q: %w", source.Name(), err)
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
			return loading.Loaded{}, fmt.Errorf("merge configuration source %q: %s", source.Name(), err)
		}
	}

	ordered := append([]compiled.Config(nil), declarations...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Type.String() < ordered[j].Type.String() })
	values := make(map[reflect.Type]reflect.Value, len(ordered))
	entries := make([]public.SnapshotEntry, 0, len(ordered))
	for _, declaration := range ordered {
		pointer := reflect.New(declaration.Type)
		if err := k.UnmarshalWithConf(declaration.Path, pointer.Interface(), koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
			return loading.Loaded{}, fmt.Errorf("decode configuration %s at %q: %s", declaration.Type, declaration.Path, err)
		}
		if err := l.validate(pointer.Interface()); err != nil {
			return loading.Loaded{}, fmt.Errorf("validate configuration %s at %q: %w", declaration.Type, declaration.Path, err)
		}
		value := pointer.Elem()
		data, err := stdjson.Marshal(value.Interface())
		if err != nil {
			return loading.Loaded{}, fmt.Errorf("snapshot configuration %s: %w", declaration.Type, err)
		}
		values[declaration.Type] = value
		entries = append(entries, public.SnapshotEntry{Type: declaration.Type, Data: data, Hash: sha256.Sum256(data)})
	}
	loadedAt := l.clock().UTC()
	return loading.Loaded{Snapshot: public.NewSnapshot(version, loadedAt, entries), Values: values}, nil
}

func (l *Loader) validate(value any) error {
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
