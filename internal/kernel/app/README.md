# internal/kernel/app

## 职责

定义 Application 状态、Observer 事件、只读 Plan 和一次性运行契约。

## 边界与失败语义

这些类型只在当前应用内部协作，不是外部 SDK。Application 状态单向推进；Observer 同步只读，
panic 会被转换为诊断错误。最终状态语义由 Runtime 统一决定。

## 关键入口

- [`State`](contracts.go)、[`Event`](contracts.go)
- [`Observer`](contracts.go)、[`Plan`](contracts.go)、[`Application`](contracts.go)

## 验证

状态、事件和重复 Run 由 [`runtime`](../../adapter/kernel/runtime/README.md)测试覆盖；精确状态速查
见[契约速查](../../../docs/reference/contracts.md)。
