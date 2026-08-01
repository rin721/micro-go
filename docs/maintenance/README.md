# Kernel 维护路线

这条路线面向修改内部 Kernel 协议、默认执行实现或架构门禁的维护者。开始前应先阅读
[架构概念](../concepts/architecture.md)，确认当前项目不是外部框架 SDK。

1. [Runtime 执行链](kernel-runtime.md)：从 Module 收集到最终关闭的状态和所有权。
2. [Adapter 与依赖边界](adapter-boundaries.md)：第三方库、Kernel Adapter 与 Capability Adapter。
3. [内部契约索引](../reference/README.md)：精确类型、状态、阶段和术语。
4. [架构决策](../decisions/README.md)：不可逆选择及后果。
5. [实现状态与演进](../roadmap/README.md)：已验证事实、限制和新能力准入条件。
6. [验证与故障定位](verification.md)：门禁、故障矩阵和事实声明方式。

修改协议与实现时必须同步迁移调用方、测试和文档，不保留旧入口、兼容别名或第二套 Runtime。
