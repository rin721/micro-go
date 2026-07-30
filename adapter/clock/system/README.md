# adapter/clock/system

本包使用标准库 `time.Now` 实现 `capability/clock.Clock`。

`Module` 注册具体 `*Clock`，绑定并导出公共接口。消费者只依赖 `clock.Clock`，因此测试可以替换为固定时钟。

Clock 无状态、无需关闭，也不提供 Timer 或 Sleep。编译期接口断言确保实现与公共契约同步。

入口见 [`system.go`](system.go)，公共契约见 [`capability/clock`](../../../capability/clock/README.md)。

