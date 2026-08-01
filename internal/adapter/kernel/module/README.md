# internal/adapter/kernel/module

## 职责

执行 `Module.Register`，并把声明转换为带模块所有权和稳定顺序的 Collection。

## 边界与失败语义

Registry 只记录意图，不解释 Provider 签名或构造实例。每个 Module 使用独立 Registry，Register
返回后立即冻结；nil、空名、重名、error 和 panic 都在资源构造前失败。

## 关键入口

- [`NewCollector`](collector.go)：创建声明收集器。
- [`Collector.Collect`](collector.go)：校验模块并冻结注册结果。

## 验证

[`collector_test.go`](collector_test.go)覆盖非法模块、panic 和冻结后注册；下游规则见
[`compiler`](../di/compiler/README.md)。
