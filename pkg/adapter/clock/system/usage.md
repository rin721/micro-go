# System Clock 使用说明

## 适用场景

System Clock 是 [`clock.Clock`](../../../../types/capability/clock/clock.go) 的生产实现，适用于业务
需要读取操作系统当前时间的场景。它只封装 `time.Now`，不提供 Timer、Sleep、虚拟时间或持久化
单调时钟。

## 接入方式

组合根注册 `system.New`，将 `*system.Clock` 绑定并导出为 `clock.Clock`。当前接入可参考
[`clockModule`](../../../../internal/bootstrap/module_clock.go)。业务构造函数只接收 Capability：

```go
func newService(appClock clock.Clock) *service {
    return &service{clock: appClock}
}
```

业务包不要直接调用 `time.Now` 或构造 `system.Clock`，否则测试无法替换时间来源。

## 配置与行为

本实现没有配置项、隐藏默认值或环境变量。`Now` 每次直接返回 `time.Now()` 的结果；返回值包含
标准库定义的墙上时间和进程内单调时间信息，但跨序列化或跨进程后不能依赖单调部分。

## 错误、并发与资源

`New` 和 `Now` 不返回错误。`Clock` 无可变状态、无 goroutine、无 I/O 资源，可以由多个组件
并发共享，也不需要进入 Runtime Close 阶段。

测试需要固定时间时，应实现一个轻量 `clock.Clock` 替身并通过构造函数注入；不要给生产 Adapter
增加测试开关或全局可变时间。

## 示例与验证

[`example_test.go`](example_test.go) 从 Capability 视角构造并读取时间。运行：

```text
go test ./pkg/adapter/clock/system -run '^Example$'
```

编译期接口断言位于 [`system.go`](system.go)，组合根边界由
[`internal/architecture`](../../../../internal/architecture/README.md)验证。
