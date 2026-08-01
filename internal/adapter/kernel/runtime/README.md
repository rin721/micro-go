# internal/adapter/kernel/runtime

## 职责

协调 Module、配置、Plan、构造、生命周期、Runner、Observer、Reload 和统一关闭。

## 边界与失败语义

Application 是唯一状态与资源所有者，只能 Run 一次。Runtime 通过 Port 接收执行部件，不自行
选择第三方实现；运行、观察和清理错误全部聚合，最终状态只能是 Closed、RestartRequired 或
Failed。所有超时为 Context 协作式预算。

Watcher 在 Runner 前建立，并通过启动前候选重读封闭 Build 到 Watch 之间的事件窗口；长期
Runner 意外正常返回属于 Failed。构造回滚由 Runtime 使用独立 shutdown budget 执行。

## 关键入口

- [`New`](dependencies.go)：校验 Collector、Compiler、Loader、Constructor、Watcher。
- [`Runtime.Compile`](compile.go)、[`Runtime.Build`](compile.go)
- [`Application.Run`](run.go)：生命周期、Reload 和关闭状态机。

## 验证

[`app_test.go`](app_test.go)覆盖图与构造，
[`lifecycle_failure_test.go`](lifecycle_failure_test.go)覆盖阶段故障，
[`reload_test.go`](reload_test.go)覆盖候选提交，`main_test.go`检查 goroutine 泄漏。整体流程见
[Runtime 执行链](../../../../docs/maintenance/kernel-runtime.md)。
