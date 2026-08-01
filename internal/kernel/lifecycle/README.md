# internal/kernel/lifecycle

## 职责

定义 `Preparer`、`Starter`、`Runner`、`Stopper`、`Closer`五个可选生命周期小接口。

## 边界与失败语义

组件只实现实际需要的阶段。全部方法必须协作响应 Context；只有 Start 成功者收到 Stop，所有
构造成功且实现 Closer 的组件都必须 Close。Runtime 负责顺序、panic 隔离和错误聚合。

## 关键入口

- [`interfaces.go`](interfaces.go)：全部生命周期契约及所有权说明。

## 验证

阶段 error、panic、超时、逆序补偿和重复 Run 由
[`lifecycle_failure_test.go`](../../adapter/kernel/runtime/lifecycle_failure_test.go)覆盖；组件选择指南见
[生命周期与 Reload](../../../docs/development/lifecycle-and-reload.md)。
