# internal/acceptance/module

## 职责

把 SQLite 配置、HTTP 配置、Store、Binding、Service 和 Server 声明为一个验收 Module。

## 边界与失败语义

Module 只登记声明，不连接数据库或监听端口。Logger、Clock 和 ID 只能通过基础 Module 导出的
Capability 注入；Capture 仅向黑盒测试暴露 Server 地址，不参与业务依赖。

## 关键入口

- [`WorkItems.Register`](module.go)：完整纵切片声明。

## 验证

Compiler 与真实构造由 [`backend_acceptance_test.go`](../../bootstrap/backend_acceptance_test.go)
的黑盒集成覆盖；权威场景见[后端纵切片验收系统](../../../docs/development/backend-acceptance.md)。
