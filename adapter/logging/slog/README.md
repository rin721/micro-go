# adapter/logging/slog

本包用标准库 `log/slog` 实现项目 `logging.Logger`，不向外暴露 `*slog.Logger`、Handler 或 Attr。

## 设计要点

- 项目 Field 在调用边界转换为 `slog.Attr`。
- `With` 与 `Named` 创建派生 Logger，但共享 LevelVar、锁和输出 owner。
- stdout/stderr 归进程所有；只有 Adapter 打开的文件由 Close 幂等释放。
- Level 通过 `slog.LevelVar` 原地更新。
- Output 或 JSON Handler 变化需要重建资源，Reload 返回 `RestartRequired`。

`Module` 声明 `logging` 配置并导出公共日志契约。行为由上层 [`contract_test.go`](../contract_test.go)验证。

