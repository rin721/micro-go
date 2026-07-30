// Package loading 定义 Application 与具体配置引擎之间的内部接口。
package loading

import (
	"context"
	"reflect"

	"github.com/rin721/micro-go/internal/di/compiled"
	"github.com/rin721/micro-go/kernel/config"
)

// Loaded 同时包含对外不可变 Snapshot 和仅供构造引擎注入的 reflect.Value。
// 二者来自同一次解码，保证组件初始值与发布快照一致。
type Loaded struct {
	// Snapshot 是可向组件 Reload 和外部诊断公开的不可变版本。
	Snapshot config.Snapshot
	// Values 是同一快照对应的构造注入值，仅在 Build 内部使用。
	Values map[reflect.Type]reflect.Value
}

// Loader 是 Application 使用的项目自有配置加载接口。
// 版本由 Application 分配，来源顺序和配置声明均由已编译计划决定。
type Loader interface {
	Load(context.Context, uint64, []config.Source, []compiled.Config) (Loaded, error)
}
