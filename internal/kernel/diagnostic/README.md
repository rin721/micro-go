# internal/kernel/diagnostic

## 职责

定义注册、配置、图编译、构造、生命周期、Reload 和 Observer 共用的阶段化错误模型。

## 边界与失败语义

`ComponentError`补充 Module、Component、Provider、Phase 并 Unwrap 原始 Cause；`PanicError`
保留 panic 值与堆栈。错误摘要不得泄露配置值或凭据，日志只在真正决定策略的边界记录。

## 关键入口

- [`Phase`](error.go)：稳定阶段集合。
- [`ComponentError`](error.go)、[`PanicError`](error.go)

## 验证

Provider、生命周期、Observer 和 Reload 的错误链由
[`runtime`](../../adapter/kernel/runtime/README.md)故障测试覆盖；整体错误路径见
[Runtime 执行链](../../../docs/maintenance/kernel-runtime.md)。
