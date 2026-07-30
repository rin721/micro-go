package loading

import (
	"context"
	"reflect"

	"github.com/rin721/micro-go/internal/di/compiled"
	"github.com/rin721/micro-go/kernel/config"
)

type Loaded struct {
	Snapshot config.Snapshot
	Values   map[reflect.Type]reflect.Value
}

// Loader 是 Application 使用的项目自有配置加载接口。
type Loader interface {
	Load(context.Context, uint64, []config.Source, []compiled.Config) (Loaded, error)
}
