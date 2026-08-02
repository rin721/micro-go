# internal/adapter/kernel/logging/slog

## 职责

提供 Kernel 必有的标准库 Slog 基线，并管理显式增强 Logger 的动态替换。

## 边界与失败语义

早期基线不依赖配置系统。Configure 失败保留原 Writer；Replace 不取得外部资源所有权；Restore
在关闭阶段恢复基线。只有本实现自己打开的文件由 Bootstrap 最终关闭。

## 关键入口

- [`New`](slog.go)：创建早期基线。
- [`Logger.Configure`](slog.go)、`Apply`：应用初始配置和 Reload。
- [`Logger.Replace`](slog.go)、`Restore`：切换与恢复 Kernel 日志目标。

## 验证

运行 `go test ./internal/adapter/kernel/logging/slog`。
