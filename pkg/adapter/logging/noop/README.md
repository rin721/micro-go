# Noop 日志 Adapter

本包提供完全静默、无状态且无资源的 `logging.Logger` 实现。

Noop 属于具体策略，因此不放在 `types/capability/logging`。`With` 和 `Named` 返回同一实例，
保持链式契约但不制造无意义分配。入口见 [`noop.go`](noop.go)。
