# Capability 设计入口

`capability` 保存业务组件可以长期依赖的小接口。它回答“业务需要什么能力”，不回答“由哪个库实现”。

## 包导航

- [logging](logging/README.md)：结构化日志、字段和 Noop。
- [clock](clock/README.md)：可替换当前时间。
- [idgen](idgen/README.md)：生成字符串 ID。

## 为什么独立于 Adapter

公共接口与具体实现分离后，业务 Provider 签名不包含 Zap、slog 或 UUID 类型；应用只需在组合根选择 [`adapter`](../adapter/README.md)。这也让测试可以使用轻量替身，而不需要启动第三方基础设施。

Capability 不负责实例选择和生命周期，唯一绑定由 [`kernel/module`](../kernel/module/README.md)声明并由 Compiler 校验。

