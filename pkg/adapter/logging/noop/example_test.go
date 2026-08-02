// 本文件演示 Noop Logger 的静默写入和无状态派生语义。
package noop_test

import (
	"context"
	"fmt"

	"github.com/rin721/micro-go/pkg/adapter/logging/noop"
	"github.com/rin721/micro-go/types/capability/logging"
)

// Example 通过项目接口派生字段和名称，并验证仍返回同一无状态实例。
func Example() {
	// 显式接口类型证明 Noop 没有要求消费者依赖具体实现。
	var logger logging.Logger = noop.New()
	// With 与 Named 对静默实现没有可观察状态，因此复用同一指针。
	derived := logger.With(logging.String("component", "test")).Named("worker")
	derived.Info(context.Background(), "discarded")

	fmt.Println(derived == logger)
	// Output: true
}
