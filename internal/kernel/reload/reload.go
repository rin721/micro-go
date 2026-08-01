// Package reload 定义不依赖具体配置实现的最小原地重载契约。
package reload

import (
	"context"

	"github.com/rin721/micro-go/internal/kernel/config"
)

// Result 描述组件处理候选配置后的决定。结果由项目定义，避免配置引擎的类型进入
// 组件契约。
type Result uint8

const (
	// Applied 表示组件已原地应用候选配置。
	Applied Result = iota
	// Ignored 表示组件确认变化无需处理。
	Ignored
	// RestartRequired 表示当前变化不能安全原地完成，应用应优雅退出并由外部重启。
	RestartRequired
)

// Reloader 是组件可选实现的最小重载接口。
// Snapshot 是经过全量加载和验证的新候选；只有所有受影响组件都接受后，Application
// 才会提升自己的当前快照。实现必须协作响应 Context 取消，并自行保证与 Runner 并发访问
// 组件状态时的同步安全。
type Reloader interface {
	Reload(context.Context, config.Snapshot) (Result, error)
}
