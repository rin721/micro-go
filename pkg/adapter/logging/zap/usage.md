# Zap Logger 使用说明

## 适用场景

Zap Adapter 使用 Uber Zap Core 实现项目
[`logging.Logger`](../../../../types/capability/logging/logging.go)。它适合应用已经明确需要 Zap 的
编码器和 AtomicLevel 行为时使用；当前 Bootstrap 默认选择的是 Slog，不会自动切换到 Zap。

## 接入方式

组合根使用 `zap.New(Config)` 构造 Logger，绑定为 `logging.Logger`，并用类似当前
[`managedLogger`](../../../../internal/bootstrap/module_logging.go) 的私有桥接翻译配置、Reload 和
Close。替换 Slog 时必须单轨修改 Bootstrap，不能同时导出两个未限定 Logger。

业务消费者只依赖 Capability。独立工具直接构造 Zap Logger 时，调用者取得关闭所有权，并应在
停止阶段调用 `Close`。

## 配置与行为

| 字段 | 合法值 | 行为 |
| --- | --- | --- |
| `Level` | Zap 支持的 Level；当前应用标签限制为 `debug`、`info`、`warn`、`error` | 最低输出级别，可原地更新 |
| `Output` | `stdout`、`stderr` 或文件路径 | 标准流或 Adapter 自有文件 |
| `Development` | `true` / `false` | Development Console Encoder 或 Production JSON Encoder |

`New` 调用 `zapcore.ParseLevel` 并打开输出，但不会主动执行 struct tag 上的 Validator 校验。通过
Kernel 配置管线时由配置所有者限制 Level；直接调用者应使用应用认可的取值并处理错误。

`Apply` 在只有 Level 变化时返回 `ChangeApplied` 并更新 `AtomicLevel`；Output 或 Development
变化返回 `ChangeRestartRequired`。当前实现为满足统一签名而接收 `Context`，但不读取或转发它。

## 错误、并发与资源

文件 Output 使用追加、创建、只写模式和 `0600` 权限。`Close` 先 Sync，再关闭 Adapter 自有
文件，并用 `errors.Join` 保留两类错误；标准流常见的无效 Sync 错误会被识别，标准流本身不关闭。

原 Logger 与 `With`/`Named` 派生 Logger 共享互斥锁、AtomicLevel 和资源 owner。`Apply` 可并发
调用，`Close` 幂等；关闭后不得继续使用任何派生 Logger。

## 示例与验证

[`example_test.go`](example_test.go) 使用临时文件验证字段写入、Level 原地应用、重启请求和资源
关闭。运行：

```text
go test ./pkg/adapter/logging/zap -run '^Example$'
```

跨实现公共行为见 [`contract_test.go`](../contract_test.go)，第三方类型隔离由
[`internal/architecture`](../../../../internal/architecture/README.md)验证。
