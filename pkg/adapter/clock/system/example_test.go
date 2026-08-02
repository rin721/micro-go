// 本文件以外部调用方视角演示 System Clock 只通过项目接口使用。
package system_test

import (
	"fmt"

	"github.com/rin721/micro-go/pkg/adapter/clock/system"
	"github.com/rin721/micro-go/types/capability/clock"
)

// Example 展示构造 System Clock 并读取非零当前时间。
func Example() {
	// 变量显式声明为 Capability，示例不会依赖 Adapter 的额外方法。
	var appClock clock.Clock = system.New()

	// 输出只验证时间有效，不绑定执行机器的实际时刻。
	fmt.Println(appClock.Now().IsZero())
	// Output: false
}
