# pkg/adapter/logging/slog

## 职责

使用标准库 `log/slog`实现项目结构化日志契约，并管理自己打开的输出资源。

## 边界与失败语义

项目 Field 只在包内转换为 `slog.Attr`。派生 Logger 共享级别、锁和资源 owner；stdout/stderr
归进程所有，文件由幂等 Close 释放。Level 可原地更新，Output 或 JSON 变化要求重启。

## 关键入口

- [`Config`](slog.go)、[`New`](slog.go)
- [`Logger.Apply`](slog.go)、[`Logger.Close`](slog.go)

## 使用说明

配置、热更新、Context 和资源所有权见[详细使用说明](usage.md)。

## 验证

[`contract_test.go`](../contract_test.go)验证公共日志行为；Bootstrap 的配置/Reload 翻译由
[`bootstrap_test.go`](../../../../internal/bootstrap/bootstrap_test.go)覆盖。
