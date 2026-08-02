# types/capability

`capability` 保存业务组件可以长期依赖的小接口。它回答“业务需要什么能力”，不回答“由哪个库实现”。

## 包导航

- [logging](logging/README.md)：结构化日志与项目 Field。
- [clock](clock/README.md)：可替换当前时间。
- [idgen](idgen/README.md)：生成字符串 ID。

## 为什么独立于 Adapter

公共接口与具体实现分离后，业务 Provider 签名不包含 Zap、slog 或 UUID 类型；应用在组合根选择
[`pkg/adapter`](../../pkg/adapter/README.md)中的普通实现。日志还由 Kernel 提供必有 Slog 基线，
增强实现必须显式切换。这也让测试可以使用轻量替身，而不需要启动第三方基础设施。

Capability 不负责实例选择和生命周期，唯一绑定由 [`internal/kernel/module`](../../internal/kernel/module/README.md)声明并由 Compiler 校验。Config、Lifecycle 和 Reload 不属于业务 Capability。
完整接入流程见[Capability 与 Adapter](../../docs/development/capability-adapters.md)。
