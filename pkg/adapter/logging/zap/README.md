# pkg/adapter/logging/zap

本包用 Uber Zap 强类型 Logger 实现项目 `logging.Logger`，第三方 `zap.Field` 只在本包内部产生。

## 设计要点

- Production 使用 JSON Encoder，Development 使用 Console Encoder。
- `zap.AtomicLevel` 支持并发安全的原地级别更新。
- 派生 Logger 共享互斥锁、AtomicLevel 和资源 owner，Close 通过 `sync.Once` 幂等。
- 标准流 Sync 的平台特定无效参数错误被忽略，但真实文件同步和关闭错误仍会返回。
- Output 或 Development 变化会替换 Core/资源，因此 `Apply` 返回 `ChangeRestartRequired`。

本包不导入 Kernel Config、Lifecycle 或 Reload。Bootstrap 可以用同样方式桥接 `Apply` 与 `Close`。行为由上层 [`contract_test.go`](../contract_test.go)验证。
