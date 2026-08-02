# types/capability/logging

## 职责

定义实现无关的结构化 `Logger`和项目自有 `Field`。

## 边界与失败语义

业务只使用普通 Go 值，不接触 `zap.Field`或`slog.Attr`。Context、消息、字段、With 和 Named
属于能力契约；资源 Close、配置 Apply 和 Reload 不进入 Logger，由 Adapter/Bootstrap 拥有。

## 关键入口

- [`Logger`](logging.go)：日志方法与派生 Logger。
- [`Field`](logging.go)及 String、Int、Bool、Duration、Time、Error 构造函数。

## 验证

[`contract_test.go`](../../../pkg/adapter/logging/contract_test.go)验证可选 Zap；Kernel 必有 Slog
实现见 [`internal/adapter/kernel/logging/slog`](../../../internal/adapter/kernel/logging/slog/README.md)，
其他业务实现见 [`pkg/adapter/logging`](../../../pkg/adapter/logging/README.md)。
