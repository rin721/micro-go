# adapter/logging

本目录是日志 Adapter 的聚合边界，并保存跨实现契约测试。

## 当前实现

- [slog](slog/README.md)：标准库实现，依赖最少。
- [zap](zap/README.md)：使用 Zap Core 和 AtomicLevel。

[`contract_test.go`](contract_test.go)用同一套断言验证字段、With、Named 和 Close 行为，确保两种实现对业务保持可替换。具体编码器、输出资源和 Reload 策略属于各自子包。

应用必须显式选择一个模块；同时注册两个实现会在图编译阶段因重复 `logging.Logger` Binding 失败。

