# cmd/app

本包是唯一进程入口。它创建可接收 Ctrl+C/SIGTERM 的根 Context，调用
`internal/bootstrap.Run`，并只在最终失败边界输出一次错误和设置退出码。

`main` 不导入 Kernel、Capability Adapter 或业务组件，也不直接 Stop/Close 资源。这个限制
保证测试可以直接驱动 Bootstrap，生命周期顺序仍只有 Runtime 一个所有者。

运行 `go run ./cmd/app`；组合方式见 [`internal/bootstrap`](../../internal/bootstrap/README.md)。
