# 类型层设计

`types` 保存跨业务包使用的稳定类型与能力契约，不保存实例、技术选型或运行时协议。

当前包含 [`capability`](capability/README.md)业务能力契约，以及仅供仓库验证使用的
[`testing`](testing/README.md)门禁类型；其 `gateconfig`包集中保存强类型策略。业务运行期的
Config、Lifecycle、Reload 和 DI 仍属于 Kernel，不迁入 types。

新契约的准入与接入方式见[Capability 封装与注入](../docs/development/capability-adapters.md)。
