# Noop Logger 使用说明

## 适用场景

Noop Logger 适用于明确不需要日志输出的进程或只关心消费者控制流的轻量测试。它实现
[`logging.Logger`](../../../../types/capability/logging/logging.go)，但丢弃所有消息和字段。

它不是生产 Logger 构造失败时的回退策略。若 Slog/Zap 因配置、路径或权限失败，应用必须保留
错误并终止启动。

## 接入方式

组合根显式注册 `noop.New` 并绑定为 `logging.Logger`。业务消费者仍只依赖 Capability，因此从
Noop 切换到资源型实现不需要修改业务签名。

测试也可以直接构造 Noop，再把它作为 `logging.Logger` 传给被测对象；不要在生产业务包内部
自行选择 Noop。

## 配置与行为

本实现没有配置项、隐藏默认值、环境变量或 Level。`Debug`、`Info`、`Warn` 和 `Error` 都立即
返回；`With` 和 `Named` 返回同一实例，不保存字段或名称。

## 错误、并发与资源

Noop 无状态、无 goroutine、无 I/O 和关闭方法，也不会返回错误。它可以并发共享，不需要
Bootstrap 生命周期桥接。

由于所有日志都会静默丢弃，选择 Noop 本身应是可审查的组合根决策，不能隐藏在通用 Helper 或
错误分支中。

## 示例与验证

[`example_test.go`](example_test.go) 演示字段、命名和写入调用可通过公共契约完成：

```text
go test ./pkg/adapter/logging/noop -run '^Example$'
```

编译期接口断言见 [`noop.go`](noop.go)。
