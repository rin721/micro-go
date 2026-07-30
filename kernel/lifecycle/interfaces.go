// Package lifecycle 定义组件可按需实现的生命周期小接口。
package lifecycle

import "context"

// Preparer 在启动外部服务前完成可失败的准备工作。
// Prepare 按依赖正序执行，失败时还没有进入 Start 的组件不会收到 Stop。
type Preparer interface {
	Prepare(context.Context) error
}

// Starter 启动组件持有的服务或后台能力。只有成功 Start 的组件才会在回滚时 Stop。
type Starter interface {
	Start(context.Context) error
}

// Runner 表示需要由 Application 并发监督的长期任务。
// 所有 Starter 成功后才会调用 Run；返回错误或意外正常返回都会触发应用关停。
type Runner interface {
	Run(context.Context) error
}

// Stopper 停止已经成功启动的组件，按依赖逆序调用，保证消费者先于依赖退出。
type Stopper interface {
	Stop(context.Context) error
}

// Closer 释放构造后持有的资源。无论组件是否成功启动，只要构造成功就会在退出或
// 构造回滚时按逆序 Close。
type Closer interface {
	Close(context.Context) error
}
