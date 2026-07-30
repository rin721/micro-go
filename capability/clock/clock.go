// Package clock 定义可替换时钟能力。
package clock

import "time"

// Clock 提供当前时间。业务依赖这个小接口而不是 time.Now，既便于测试替换，也避免
// 把具体时钟实现与 DI、生命周期绑定在一起。
type Clock interface{ Now() time.Time }
