// 本文件定义 Runtime 对模块、编译、配置、构造、监听和日志能力的显式内部端口。
package runtime

import (
	"context"
	"errors"
	"reflect"

	"github.com/rin721/micro-go/internal/adapter/kernel/di/compiled"
	registration "github.com/rin721/micro-go/internal/adapter/kernel/module"
	"github.com/rin721/micro-go/internal/kernel/config"
	kernellogging "github.com/rin721/micro-go/internal/kernel/logging"
	"github.com/rin721/micro-go/internal/kernel/module"
)

// CollectorPort 把 Module 注册转换为带所有权和稳定顺序的声明集合。
type CollectorPort interface {
	// Collect 同步执行模块注册并返回冻结声明集合。
	Collect([]module.Module) (registration.Collection, error)
}

// CompilerPort 负责项目自己的可见性、唯一绑定、循环和拓扑顺序规则。
type CompilerPort interface {
	// Compile 静态校验声明并生成稳定执行计划。
	Compile(registration.Collection) (*compiled.Plan, error)
}

// LoaderPort 负责从全新候选配置树生成强类型不可变快照。
type LoaderPort interface {
	// Load 从完整来源集合生成指定版本的强类型配置结果。
	Load(context.Context, uint64, []config.Source, []compiled.Config) (config.Loaded, error)
}

// ConstructorPort 只执行已编译计划，不拥有依赖图规则。
type ConstructorPort interface {
	// Construct 按计划构造实例，并在失败时返回已完成的可回滚前缀。
	Construct(context.Context, *compiled.Plan, map[reflect.Type]reflect.Value) ([]compiled.Instance, error)
}

// WatcherPort 把底层文件事件转换为 Kernel 自有 Change。
type WatcherPort interface {
	// Watch 启动由 Context 管理的变化和错误通道。
	Watch(context.Context, []config.Source) (<-chan config.Change, <-chan error, error)
}

// Dependencies 是 Bootstrap 必须显式提供的默认 Runtime 执行部件。
// 显式注入避免 Runtime 为方便而自行创建 Dig、Koanf 或 fsnotify 实现。
type Dependencies struct {
	// Logger 提供从注册开始即存在的 Kernel 日志，并管理显式增强实现的切换。
	Logger kernellogging.Manager
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
type Runtime struct {
	// dependencies 是 Bootstrap 在 New 时一次性提供并验证的执行部件集合。
	dependencies Dependencies
}

// New 创建默认运行时，并在任何模块注册或资源构造前校验装配完整性。
func New(dependencies Dependencies) (*Runtime, error) {
	// 按 Runtime 首次使用顺序逐项检查，返回最直接的缺失装配原因。
	switch {
	case dependencies.Logger == nil || isNilKernelLogger(dependencies.Logger):
		return nil, errors.New("runtime logger is nil")
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
		// 所有依赖通过后按值保存接口集合，运行期不提供重新装配入口。
		return &Runtime{dependencies: dependencies}, nil
	}
}

// isNilKernelLogger 检测接口中包裹的 nil 引用，避免日志首调用时才发生 panic。
func isNilKernelLogger(logger kernellogging.Manager) bool {
	// 只有语言允许为 nil 的 Kind 才调用 IsNil，其他值实现直接视为有效。
	value := reflect.ValueOf(logger)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
