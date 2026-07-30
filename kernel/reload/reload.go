// Package reload 定义不依赖具体配置实现的最小原地重载契约。
package reload

import (
	"context"

	"github.com/rin721/micro-go/kernel/config"
)

type Result uint8

const (
	Applied Result = iota
	Ignored
	RestartRequired
)

type Reloader interface {
	Reload(context.Context, config.Snapshot) (Result, error)
}
