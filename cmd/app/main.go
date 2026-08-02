// Package main 是单进程应用唯一的可执行入口。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rin721/micro-go/internal/bootstrap"
)

// main 建立根信号 Context，运行唯一组合根，并把最终结果映射为进程退出码。
func main() {
	// 信号只负责取消根 Context；组件停止顺序和资源释放统一由 Runtime 决定，
	// 避免 main 与生命周期协调器同时关闭同一资源。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// 无论启动成功还是失败，退出入口时都注销信号订阅，避免标准库继续持有通知资源。
	defer stop()

	// Bootstrap 拥有完整运行期；main 只把根 Context 交给它并处理最终退出码。
	if err := bootstrap.Run(ctx); err != nil {
		// 终端边界只记录一次最终错误，内部各层保留错误链但不重复打印。
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			// 连最终错误都无法写入标准错误时使用独立退出码，便于外部监督器区分诊断失败。
			os.Exit(2)
		}
		// Bootstrap 返回错误代表应用未能正常完成，使用通用失败退出码通知进程监督器。
		os.Exit(1)
	}
}
