// 本文件展示业务辅助函数只接收项目 Logger，并可替换任意 Adapter。
package logging_test

import (
	"context"
	"fmt"

	"github.com/rin721/micro-go/pkg/adapter/logging/noop"
	"github.com/rin721/micro-go/types/capability/logging"
)

// logReady 记录一条带命名空间和结构化字段的就绪事件。
func logReady(logger logging.Logger) {
	logger.Named("process").Info(
		context.Background(),
		"ready",
		logging.String("component", "application.process"),
		logging.Bool("running", true),
	)
}

// Example 使用 Noop 证明消费者逻辑不需要知道具体日志实现。
func Example() {
	// Noop 满足同一契约，调用业务辅助函数不会产生外部副作用。
	logReady(noop.New())

	fmt.Println("consumer only depends on logging.Logger")
	// Output: consumer only depends on logging.Logger
}
