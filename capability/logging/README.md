# capability/logging

本包定义实现无关的结构化日志契约和项目自有 `Field`。

## 为什么这样设计

业务如果直接使用 `zap.Field` 或 `slog.Attr`，更换实现就会扩散到所有 Provider 和调用点。项目 Field 只保存键和值，由 Adapter 在最后一层转换。

## 契约

- `Debug`、`Info`、`Warn`、`Error` 接收 Context、消息和字段。
- `With` 返回带固定字段的派生 Logger。
- `Named` 创建逻辑命名空间。
- `Noop` 为可选场景提供零副作用实现。

当前 [Slog](../../adapter/logging/slog/README.md) 与 [Zap](../../adapter/logging/zap/README.md) 共享同一契约测试。日志资源关闭和配置 Reload 属于具体 Adapter，不进入 Logger 接口。

