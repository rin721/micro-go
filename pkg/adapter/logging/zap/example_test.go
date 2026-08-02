// 本文件演示 Zap 文件输出、派生字段、级别原地更新和重启判定。
package zap_test

import (
	"context"
	"fmt"
	"os"
	"strings"

	zapadapter "github.com/rin721/micro-go/pkg/adapter/logging/zap"
	"github.com/rin721/micro-go/types/capability/logging"
)

// Example 走通 Zap Adapter 的创建、写入、Reload 判断、关闭和内容验证。
func Example() {
	// 先创建并关闭临时文件句柄，把后续追加写所有权交给 Adapter。
	file, err := os.CreateTemp("", "micro-go-zap-*.log")
	if err != nil {
		panic(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		panic(err)
	}
	defer os.Remove(path)

	// 初始 info 配置创建生产 JSON Core，并写入派生 Logger 事件。
	config := zapadapter.Config{Level: "info", Output: path}
	logger, err := zapadapter.New(config)
	if err != nil {
		panic(err)
	}
	logger.Named("worker").With(logging.String("component", "example")).Info(context.Background(), "started")

	// 仅改变 Level 可以原地应用，随后 Debug 消息应变为可见。
	config.Level = "debug"
	applied, err := logger.Apply(config)
	if err != nil {
		panic(err)
	}
	logger.Debug(context.Background(), "debug-enabled")

	// 改变 Development 会更换 Encoder，当前实例只能返回 RestartRequired。
	restartConfig := config
	restartConfig.Development = true
	restart, err := logger.Apply(restartConfig)
	if err != nil {
		panic(err)
	}
	if err := logger.Close(context.Background()); err != nil {
		panic(err)
	}
	// 关闭完成后读取文件，确保缓冲已同步且两条预期消息存在。
	content, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	fmt.Println(applied == zapadapter.ChangeApplied, restart == zapadapter.ChangeRestartRequired)
	fmt.Println(strings.Contains(string(content), "started"), strings.Contains(string(content), "debug-enabled"))
	// Output:
	// true true
	// true true
}
