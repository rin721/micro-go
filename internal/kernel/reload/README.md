# internal/kernel/reload

## 职责

定义组件处理已验证配置候选时的最小 `Reloader` 契约和结果集合。

## 边界与失败语义

`Applied`表示已应用，`Ignored`表示确认无需处理，`RestartRequired`表示应清理并退出。实现必须
响应 Context 并同步保护与 Runner 共享的状态；本契约不承诺跨组件回滚。

## 关键入口

- [`Result`](reload.go)、[`Reloader`](reload.go)
- [ADR-0003](../../../docs/decisions/adr-0003-reload-failure-exit.md)：失败退出决策。

## 验证

候选保留、版本提升、RestartRequired、部分应用失败和超时由
[`reload_test.go`](../../adapter/kernel/runtime/reload_test.go)覆盖。
