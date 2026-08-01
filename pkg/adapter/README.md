# pkg/adapter

`pkg/adapter` 只保存业务 Capability 的具体实现。第三方库可以在这里使用，但导出签名不得
出现第三方类型，也不得依赖内部 Kernel 生命周期。

## 包导航

- [clock/system](clock/system/README.md)：标准库系统时钟。
- [idgen/uuid](idgen/uuid/README.md)：Google UUID 字符串生成器。
- [logging](logging/README.md)：日志适配器共享契约测试。
- [logging/slog](logging/slog/README.md)：标准库 Slog 实现。
- [logging/zap](logging/zap/README.md)：Uber Zap 实现。

## 选择原则

应用在组合根显式选择 Adapter，Runtime 不会自动挑选。Capability Adapter 不导入 Kernel；
生命周期和 Reload 由 Bootstrap 私有桥接。Dig、Koanf 和 fsnotify 位于
[`internal/adapter/kernel`](../../internal/adapter/kernel/README.md)，只实现内部协议。
