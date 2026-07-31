// Package system 提供系统时钟适配器。
package system

import (
	"time"

	clockcontract "github.com/rin721/micro-go/types/capability/clock"
)

// Clock 使用操作系统墙上时钟实现 capability/clock.Clock。
type Clock struct{}

// New 创建无状态系统时钟。
func New() *Clock { return &Clock{} }

// Now 返回 time.Now 的当前结果。
func (*Clock) Now() time.Time { return time.Now() }

// 编译期断言确保 Adapter 始终满足项目契约，不需要在运行期做能力探测。
var _ clockcontract.Clock = (*Clock)(nil)
