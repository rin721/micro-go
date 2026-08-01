# internal/acceptance/workitems

## 职责

定义后端纵切片的业务模型、Repository/Readiness 消费者契约和应用服务。

## 边界与失败语义

本包不导入 HTTP、SQLite 或 Kernel。标题规则在调用 Repository 前执行；持久化错误保留原因链，
稳定的 NotFound/InvalidTitle 可由传输层使用 `errors.Is`映射。

## 关键入口

- [`Service`](workitems.go)：创建、查询和幂等完成 Work Item。
- [`Repository`](workitems.go)：应用服务拥有的持久化契约。

## 验证

[`workitems_test.go`](workitems_test.go)覆盖标题规则、时间/ID 注入和持久化调用边界；真实协议见
[后端纵切片验收系统](../../../docs/development/backend-acceptance.md)。
