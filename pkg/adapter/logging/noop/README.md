# pkg/adapter/logging/noop

## 职责

提供完全静默的 `logging.Logger`具体实现。

## 边界与失败语义

Noop 无状态、无资源且不返回错误。`With`和`Named`返回同一实例；它是显式策略，不是
Capability 的默认行为，也不进入 Kernel 生命周期。

## 关键入口

- [`New`](noop.go)：创建静默 Logger。
- [`Logger`](noop.go)：丢弃全部日志调用。

## 使用说明

显式选择、行为限制和测试用法见[详细使用说明](usage.md)。

## 验证

编译期接口断言保证实现匹配 [`logging.Logger`](../../../../types/capability/logging/README.md)；
选择具体日志策略必须发生在 Bootstrap。
