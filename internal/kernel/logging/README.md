# internal/kernel/logging

## 职责

定义 Kernel 必有日志的管理契约，并复用业务侧 `logging.Logger` 作为可替换能力。

## 边界与失败语义

Manager 始终保留基线 Logger；替换只改变委托目标，不取得外部 Logger 的关闭所有权。Kernel
恢复基线后，外部实例仍由声明它的 Module 和 Runtime 生命周期释放。

## 关键入口

- [`Manager`](logging.go)：记录日志并显式 Replace、Restore。

## 验证

默认实现和切换行为由 [`internal/adapter/kernel/logging/slog`](../../adapter/kernel/logging/slog)验证。
