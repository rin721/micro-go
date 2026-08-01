# pkg/adapter/logging

## 职责

聚合日志 Adapter，并从公共 Capability 视角维护跨实现契约测试。

## 边界与失败语义

各实现不声明 Kernel Module；Bootstrap 选择唯一实现并桥接资源与 Reload。Slog、Zap 必须一致
支持字段、With、Named 和幂等 Close，具体编码器与资源策略仍属于各自子包。

## 关键入口

- [`slog`](slog/README.md)、[`zap`](zap/README.md)、[`noop`](noop/README.md)
- [`contract_test.go`](contract_test.go)：Slog/Zap 共享行为断言。

## 验证

运行 `go test ./pkg/adapter/logging`执行跨实现契约；选择和桥接流程见
[Capability 与 Adapter](../../../docs/development/capability-adapters.md)。
