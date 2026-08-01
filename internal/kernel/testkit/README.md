# internal/kernel/testkit

## 职责

提供仅供 Kernel 测试使用的并发安全 Observer 记录工具。

## 边界与失败语义

本包不参与生产运行，不提供容器查询、实例替换或绕过模块边界的测试后门。事件切片以副本
返回；Runner 泄漏检查仍由测试包结合 `goleak`负责。

## 关键入口

- [`RecorderObserver`](observer.go)：记录并复制返回 Application 事件。

## 验证

该工具由 Runtime 的 Observer 与 Reload 测试间接覆盖；文档中的故障矩阵见
[验证与故障定位](../../../docs/maintenance/verification.md)。
