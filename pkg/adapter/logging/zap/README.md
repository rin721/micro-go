# pkg/adapter/logging/zap

## 职责

使用 Uber Zap 实现项目结构化日志契约，并封装 Core、AtomicLevel 和输出资源。

## 边界与失败语义

`zap.Field`不离开本包。派生 Logger 共享锁、级别和资源 owner；Close 幂等并保留真实文件同步/
关闭错误。Level 可原地更新，Output 或 Development 变化要求重启。

## 关键入口

- [`Config`](zap.go)、[`New`](zap.go)
- [`Logger.Apply`](zap.go)、[`Logger.Close`](zap.go)

## 使用说明

配置、热更新、Context 差异和资源所有权见[详细使用说明](usage.md)。

## 验证

[`contract_test.go`](../contract_test.go)与 Slog 使用同一行为断言；第三方导出面由
[`internal/architecture`](../../../../internal/architecture/README.md)检查。
