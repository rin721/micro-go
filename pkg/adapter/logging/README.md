# pkg/adapter/logging

本目录是日志 Adapter 的聚合边界，并保存跨实现契约测试。

## 当前实现

- [slog](slog/README.md)：标准库实现，依赖最少。
- [zap](zap/README.md)：使用 Zap Core 和 AtomicLevel。
- [noop](noop/README.md)：零副作用静默实现。

[`contract_test.go`](contract_test.go)用同一套断言验证字段、With、Named 和 Close 行为，确保两种实现对业务保持可替换。具体编码器、输出资源和配置调整策略属于各自子包。

各实现不声明 Kernel Module。Bootstrap 选择一个实现并建立 Binding，防止具体 Adapter 与应用运行协议耦合。
