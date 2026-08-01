# pkg/adapter/clock/system

## 职责

使用标准库 `time.Now`实现项目 `clock.Clock`。

## 边界与失败语义

本包无状态、无资源且不导入 Kernel。它只读取当前时间，不承诺 Timer、Sleep 或可单调持久化
时钟；Provider、Binding 和 Export 由 Bootstrap 声明。

## 关键入口

- [`New`](system.go)：创建 System Clock。
- [`Clock.Now`](system.go)：返回当前时间。

## 使用说明

构造、注入和测试替换方式见[详细使用说明](usage.md)。

## 验证

编译期接口断言保证实现匹配 [`clock.Clock`](../../../../types/capability/clock/README.md)；接入方式
见[Capability 与 Adapter](../../../../docs/development/capability-adapters.md)。
