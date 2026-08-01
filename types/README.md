# 类型层设计

`types` 保存跨业务包使用的稳定类型与能力契约，不保存实例、技术选型或运行时协议。

当前唯一领域是 [`capability`](capability/README.md)。Config、Lifecycle、Reload 和 DI 属于
Kernel 运行语义，刻意不迁入 types，避免后续维护者把内部协调协议误认为业务公共类型。
新契约的准入与接入方式见[Capability 与 Adapter](../docs/development/capability-adapters.md)。
