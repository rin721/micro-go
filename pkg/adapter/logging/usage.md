# Logging Adapter 使用说明

## 适用场景

本目录的实现共同满足项目 [`logging.Logger`](../../../types/capability/logging/logging.go) 契约。
业务代码使用 `Field`、`With` 和 `Named` 表达结构化上下文，组合根根据应用需要选择唯一实现。

| 实现 | 输出 | 配置变化 | Context | 典型用途 |
| --- | --- | --- | --- | --- |
| [Noop](noop/usage.md) | 丢弃 | 无 | 不读取 | 显式静默或轻量测试 |
| [Zap](zap/usage.md) | Console/JSON、标准流或文件 | Level 原地应用 | 当前不读取 | 需要 Zap Core 的应用 |

## 接入方式

消费者构造函数接收 `logging.Logger`，不要接收 Slog、Zap 或具体 Adapter 类型：

```go
func newService(logger logging.Logger) *service {
    return &service{logger: logger.Named("service")}
}
```

使用 `With` 添加共享字段，使用 `Named` 添加组件命名空间；两者返回的派生 Logger 与原 Logger
共享底层资源。默认 Kernel Slog 与可选 Zap 的选择差异、配置桥接和最终注入位置见
[Capability 封装与注入](../../../docs/development/capability-adapters.md#logging配置和生命周期桥接)。

## 配置与行为

公共契约提供 `String`、`Int`、`Bool`、`Duration`、`Time` 和 `Error` 字段构造函数。字段值由
具体 Adapter 编码，业务层不导入 `slog.Attr` 或 `zap.Field`。

Capability 不定义默认实现、输出格式、最低 Level、Reload 或 Close。它们属于具体 Adapter 和
组合根策略；选择实现前必须阅读对应使用说明。

## 错误、并发与资源

日志写入方法没有错误返回值，因此构造、配置应用和关闭阶段必须保留可返回的错误。业务代码不
负责关闭注入的共享 Logger；资源所有者是组合根和 Runtime 生命周期。

Noop 必须由组合根显式选择，不能在 Kernel Slog 或 Zap 构造失败时作为静默回退。否则文件权限、配置或
资源错误会被掩盖。

## 示例与验证

[`example_test.go`](example_test.go) 演示业务只依赖 Capability；
[`contract_test.go`](contract_test.go) 验证 Zap 的字段、派生 Logger 和幂等 Close；Kernel Slog
由其内部包测试覆盖动态切换、配置和资源行为。运行：

```text
go test ./pkg/adapter/logging/...
```
