// Package constructor 定义 Application 与具体 DI 构造引擎之间的内部适配接口。
package constructor

import (
	"context"
	"reflect"

	"github.com/rin721/micro-go/internal/di/compiled"
)

// Engine 是 Application 使用的项目自有构造引擎接口。
// Context、compiled.Plan 和项目 Instance 构成完整边界，因此 app 不需要导入 Dig。
type Engine interface {
	Construct(context.Context, *compiled.Plan, map[reflect.Type]reflect.Value) ([]compiled.Instance, error)
}
