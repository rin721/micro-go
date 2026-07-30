package constructor

import (
	"context"
	"reflect"

	"github.com/rin721/micro-go/internal/di/compiled"
)

// Engine 是 Application 使用的项目自有构造引擎接口。
type Engine interface {
	Construct(context.Context, *compiled.Plan, map[reflect.Type]reflect.Value) ([]compiled.Instance, error)
}
