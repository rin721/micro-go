# 文档中心

本文档中心提供两条共享主题页的阅读路线，不维护彼此复制的两套手册。

## 应用开发路线

1. [开始运行](getting-started/README.md)
2. [应用开发入口](development/README.md)
3. [组件接入工作流](development/component-workflow.md)
4. [配置开发](development/configuration.md)
5. [生命周期与 Reload](development/lifecycle-and-reload.md)
6. [Capability 与 Adapter](development/capability-adapters.md)
7. [验证与故障定位](maintenance/verification.md)

## Kernel 维护路线

1. [架构概念](concepts/README.md)
2. [Kernel 维护入口](maintenance/README.md)
3. [Runtime 执行链](maintenance/kernel-runtime.md)
4. [Adapter 与依赖边界](maintenance/adapter-boundaries.md)
5. [内部契约索引](reference/README.md)
6. [架构决策](decisions/README.md)
7. [当前状态与演进](roadmap/README.md)
8. [验证与故障定位](maintenance/verification.md)

## 文档职责

| 类型 | 回答的问题 | 权威位置 |
| --- | --- | --- |
| Getting Started | 怎样运行并确认脚手架正常工作 | [`getting-started`](getting-started/README.md) |
| Development | 怎样实现、注册、配置和接入组件 | [`development`](development/README.md) |
| Concepts | 为什么采用当前架构 | [`concepts`](concepts/README.md) |
| Maintenance | Kernel 怎样执行、怎样定位失败 | [`maintenance`](maintenance/README.md) |
| Reference | 精确名称、状态和契约是什么 | [`reference`](reference/README.md) |
| Decisions | 哪些难以逆转的选择已经确认 | [`decisions`](decisions/README.md) |
| Roadmap | 已实现什么、限制和准入方向是什么 | [`roadmap`](roadmap/README.md) |

包级 README 只解释相邻源码的职责、边界和验证入口，不复制主题文档。源码和测试是行为事实
来源；文档若与实现冲突，应先按[验证流程](maintenance/verification.md)确认事实再修正文档。
