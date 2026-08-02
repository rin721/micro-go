# Slog Logger 使用说明

## 适用场景

Slog Adapter 使用 Go 标准库 `log/slog` 实现项目
[`logging.Logger`](../../../../types/capability/logging/logging.go)。它适合希望优先采用标准库、需要
Text/JSON 输出和运行期 Level 调整的应用，也是当前 Bootstrap 选择的日志实现。

## 接入方式

组合根使用 `slog.New(Config)` 构造 Logger，再绑定为 `logging.Logger`。当前
[`managedLogger`](../../../../internal/bootstrap/module_logging.go) 负责把应用配置和 Kernel Reload
转换为本包类型，并把 Close 交给 Runtime。

业务消费者只调用 Capability 的日志方法、`With` 和 `Named`，不得持有 `*slog.Logger`、调用
`Apply` 或关闭共享 Logger。独立工具若直接构造 Logger，则自身必须负责 Close。

## 配置与行为

| 字段 | 合法值 | 行为 |
| --- | --- | --- |
| `Level` | `debug`、`info`、`warn`、`error`，忽略大小写 | 最低输出级别，可原地更新 |
| `Output` | `stdout`、`stderr` 或文件路径 | 标准流或 Adapter 自有文件 |
| `JSON` | `true` / `false` | JSON Handler 或 Text Handler |

`New` 会解析 Level 并打开输出，但不会主动执行 struct tag 上的完整 Validator 校验。通过 Kernel
配置加载时由配置管线校验标签；直接调用者必须提供完整配置并处理构造错误。

`Apply` 在只有 Level 变化时返回 `ChangeApplied` 并通过 `slog.LevelVar` 更新；Output 或 JSON
变化返回 `ChangeRestartRequired`，不会修改当前 Logger。Slog 把调用方 `Context` 传入 Handler。

## 错误、并发与资源

非标准流 Output 以追加、创建、只写模式和 `0600` 权限打开。文件由 Logger 所有；stdout 和
stderr 归进程所有，不会被关闭。打开文件失败和非法 Level 会保留错误返回给调用方。

原 Logger 与 `With`/`Named` 派生 Logger 共享 Level、互斥锁和资源 owner。`Apply` 可并发调用，
`Close` 通过 `sync.Once` 幂等关闭自有文件。资源关闭后不得继续写日志。

## 示例与验证

[`example_test.go`](example_test.go) 使用临时文件验证结构化写入、Level 原地应用、重启请求和
关闭语义。运行：

```text
go test ./pkg/adapter/logging/slog -run '^Example$'
```

跨实现行为由 [`contract_test.go`](../contract_test.go)验证；当前 Bootstrap 翻译由
[`bootstrap_test.go`](../../../../internal/bootstrap/bootstrap_test.go)验证。
