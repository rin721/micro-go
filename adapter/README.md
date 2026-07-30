# Adapter 设计入口

`adapter` 保存公共 Capability 的具体实现。第三方库可以在这里使用，但导出签名不得出现第三方类型。

## 包导航

- [clock/system](clock/system/README.md)：标准库系统时钟。
- [idgen/uuid](idgen/uuid/README.md)：Google UUID 字符串生成器。
- [logging](logging/README.md)：日志适配器共享契约测试。
- [logging/slog](logging/slog/README.md)：标准库 Slog 实现。
- [logging/zap](logging/zap/README.md)：Uber Zap 实现。

## 选择原则

应用在组合根显式选择 Adapter，框架不会自动挑选。当 Slog 和 Zap 同时绑定 `logging.Logger` 时，Compiler 报告唯一绑定冲突，避免环境差异导致隐式选择。

Dig、Koanf 和 fsnotify 是框架内部执行引擎，位于 [`internal`](../internal/README.md)，不是业务 Capability Adapter。

