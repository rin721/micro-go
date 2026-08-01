# internal/bootstrap

## 职责

唯一组合根：选择 Kernel/Capability Adapter、配置来源、Module、超时和默认业务 Runner。

## 边界与失败语义

业务组件不承担装配；Adapter 也不反向导入 Kernel Module。`Run`返回前已完成 Stop、等待 Runner
和 Close。配置或构造失败发生在业务运行前，运行错误完整返回进程入口。

## 关键入口

- [`Run`](bootstrap.go)：构造默认 Runtime 并驱动 Application。
- [`loggingModule`](bootstrap.go)、`clockModule`、`idModule`、`applicationModule`：当前模块集合。

## 验证

[`bootstrap_test.go`](bootstrap_test.go)使用真实默认栈验证启动、取消、缺失配置和日志 Reload；
应用接入流程见[组件接入工作流](../../docs/development/component-workflow.md)。
