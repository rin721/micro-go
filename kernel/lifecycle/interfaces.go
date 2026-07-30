// Package lifecycle 定义组件可按需实现的生命周期小接口。
package lifecycle

import "context"

type Preparer interface {
	Prepare(context.Context) error
}

type Starter interface {
	Start(context.Context) error
}

type Runner interface {
	Run(context.Context) error
}

type Stopper interface {
	Stop(context.Context) error
}

type Closer interface {
	Close(context.Context) error
}
