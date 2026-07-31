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

func main() {
	// 信号只负责取消根 Context；组件停止顺序和资源释放统一由 Runtime 决定，
	// 避免 main 与生命周期协调器同时关闭同一资源。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := bootstrap.Run(ctx); err != nil {
		// 终端边界只记录一次最终错误，内部各层保留错误链但不重复打印。
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
