# internal/kernel/app

本包定义 Application 的单向状态、Observer 事件、只读 Plan 和一次性运行契约。

这些类型留在 internal，是因为脚手架维护者需要它们协调默认实现，但外部项目不应把它们
当成需要长期兼容的框架 API。具体 Compile、Build、Runner 监督和关闭实现位于
[`pkg/adapter/kernel/runtime`](../../../pkg/adapter/kernel/runtime/README.md)。

状态只能沿构造、启动、运行和关闭方向推进；同一 Application 只能 Run 一次。测试重点见
Runtime 的 `app_test.go` 与 `reload_test.go`。
