# pkg/adapter/clock/system

本包使用标准库 `time.Now` 实现 `capability/clock.Clock`。

本包只提供具体 `*Clock`。Provider、Binding 和 Export 由 Bootstrap 声明，避免 Adapter 反向依赖 Kernel Module。

Clock 无状态、无需关闭，也不提供 Timer 或 Sleep。编译期接口断言确保实现与公共契约同步。

入口见 [`system.go`](system.go)，公共契约见 [`types/capability/clock`](../../../../types/capability/clock/README.md)。
