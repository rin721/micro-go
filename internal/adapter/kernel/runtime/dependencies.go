package runtime

import (
	"context"
	"errors"
	"reflect"

	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiled"
	registration "github.com/rin721/micro-go/internal/adapter/kernel/module"
	"github.com/rin721/micro-go/internal/kernel/config"
	"github.com/rin721/micro-go/internal/kernel/module"
)

// CollectorPort 把 Module 注册转换为带所有权和稳定顺序的声明集合。
type CollectorPort interface {
	Collect([]module.Module) (registration.Collection, error)
}

// CompilerPort 负责项目自己的可见性、唯一绑定、循环和拓扑顺序规则。
type CompilerPort interface {
	Compile(registration.Collection) (*compiled.Plan, error)
}

// LoaderPort 负责从全新候选配置树生成强类型不可变快照。
type LoaderPort interface {
	Load(context.Context, uint64, []config.Source, []compiled.Config) (config.Loaded, error)
}

// ConstructorPort 只执行已编译计划，不拥有依赖图规则。
type ConstructorPort interface {
	Construct(context.Context, *compiled.Plan, map[reflect.Type]reflect.Value) ([]compiled.Instance, error)
}

// WatcherPort 把底层文件事件转换为 Kernel 自有 Change。
type WatcherPort interface {
	Watch(context.Context, []config.Source) (<-chan config.Change, <-chan error, error)
}

// Dependencies 是 Bootstrap 必须显式提供的默认 Runtime 执行部件。
// 显式注入避免 Runtime 为方便而自行创建 Dig、Koanf 或 fsnotify 实现。
type Dependencies struct {
	// Collector 执行模块注册和声明冻结。
	Collector CollectorPort
	// Compiler 执行项目依赖图规则。
	Compiler CompilerPort
	// Loader 生成已验证候选 Snapshot。
	Loader LoaderPort
	// Constructor 按冻结计划构造实例。
	Constructor ConstructorPort
	// Watcher 把文件事件转换为 Kernel Change。
	Watcher WatcherPort
}

// Runtime 协调 Kernel 状态机，但不决定各执行部件采用的第三方技术栈。
type Runtime struct{ dependencies Dependencies }

// New 创建默认运行时，并在任何模块注册或资源构造前校验装配完整性。
func New(dependencies Dependencies) (*Runtime, error) {
	switch {
	case dependencies.Collector == nil:
		return nil, errors.New("runtime collector is nil")
	case dependencies.Compiler == nil:
		return nil, errors.New("runtime compiler is nil")
	case dependencies.Loader == nil:
		return nil, errors.New("runtime config loader is nil")
	case dependencies.Constructor == nil:
		return nil, errors.New("runtime constructor is nil")
	case dependencies.Watcher == nil:
		return nil, errors.New("runtime config watcher is nil")
	default:
		return &Runtime{dependencies: dependencies}, nil
	}
}
