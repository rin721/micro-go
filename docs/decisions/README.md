# 架构决策

ADR 记录已经接受且难以逆转的选择。后续若改变结论，应新增 ADR 取代旧决策，不直接把历史
改写成当前说明。

| ADR | 状态 | 决策 |
| --- | --- | --- |
| [ADR-0001](adr-0001-third-party-boundary.md) | Accepted | 第三方库只能实现项目契约 |
| [ADR-0002](adr-0002-single-process-scaffold.md) | Accepted | 定位为单进程 Go 应用脚手架 |
| [ADR-0003](adr-0003-reload-failure-exit.md) | Accepted | Reload 采用失败退出模型 |
| [ADR-0004](adr-0004-kernel-logging.md) | Accepted | 日志是唯一双阶段能力 |

当前架构说明见[架构概念](../concepts/architecture.md)，已实现程度见
[实现状态](../roadmap/implementation-status.md)。
