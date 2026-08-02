# pkg/adapter/logging

## 职责

聚合日志 Adapter，并从公共 Capability 视角维护跨实现契约测试。

## 边界与失败语义

本目录只保留可选业务实现，不声明 Kernel Module。Zap 支持字段、With、Named、配置 Reload
和幂等 Close；Noop 只用于显式静默或测试。Kernel 必有的 Slog 基线位于内部 Kernel Adapter，
不以公共业务 Adapter 形成第二套实现。

## 关键入口

- [`zap`](zap/README.md)、[`noop`](noop/README.md)
- [`contract_test.go`](contract_test.go)：验证可选 Zap 满足公共 Logger 契约。

## 使用说明

字段用法、实现选择和共同接入方式见[详细使用说明](usage.md)。

## 验证

运行 `go test ./pkg/adapter/logging`执行跨实现契约；选择和桥接流程见
[Capability 封装与注入](../../../docs/development/capability-adapters.md)。
