# internal/acceptance/transport/workitemshttp

## 职责

提供 Work Item 验收系统的标准库 HTTP Adapter、健康投影和稳定 JSON 错误边界。

## 边界与失败语义

Server 限制请求体并拒绝未知 JSON 字段，只依赖应用 Service 和 Readiness，不查询 Runtime 或
SQLite。内部错误记录一次并返回固定 500 文本；资源由 Prepare/Start/Run/Stop/Close 管理。

## 关键入口

- [`New`](server.go)：构造路由。
- [`Server.Prepare`](server.go)、[`Server.Run`](server.go)、[`Server.Stop`](server.go)
- `/livez`、`/readyz`、`/status`和 `/v1/work-items`路由。

## 验证

[`server_test.go`](server_test.go)验证在途请求 drain；HTTP 与持久化黑盒证据位于
[`backend_acceptance_test.go`](../../../bootstrap/backend_acceptance_test.go)，真实 Unix 子进程
SIGTERM 门禁位于 [`backend_process_unix_test.go`](../../../bootstrap/backend_process_unix_test.go)。
