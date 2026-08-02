# pkg/adapter 使用中心

`pkg/adapter` 保存业务 Capability 的具体实现。业务组件只依赖
[`types/capability`](../../types/capability/README.md)，组合根负责选择并构造这里的实现；Runtime
不会自动扫描或挑选 Adapter。

## 阅读路线

1. 先阅读[组合根接入](integration.md)，确认依赖方向和资源所有权。
2. 按需要进入 Clock、ID 或 Logging 的使用说明。
3. 从使用说明链接到可编译 Example 和实现源码，验证文档与 API 一致。

## 当前实现

| Capability | 实现 | 选择依据 | 详细说明 |
| --- | --- | --- | --- |
| Clock | [System Clock](clock/system/README.md) | 使用操作系统当前时间 | [使用说明](clock/system/usage.md) |
| ID Generator | [Google UUID](idgen/uuid/README.md) | 生成不透明字符串 UUID | [使用说明](idgen/uuid/usage.md) |
| Logging | [Noop](logging/noop/README.md) | 显式静默策略 | [使用说明](logging/noop/usage.md) |
| Logging | [Zap](logging/zap/README.md) | Zap Core 与 AtomicLevel | [使用说明](logging/zap/usage.md) |

日志包边界见 [`logging/README.md`](logging/README.md)，字段用法和实现选择矩阵见
[Logging 使用说明](logging/usage.md)。各包的简短 `README.md` 只描述局部边界，不代替对应的
`usage.md`。

## 选择与所有权

- 业务 Provider 的参数使用 Capability，不导入具体 Adapter。
- Bootstrap 是唯一同时选择 Kernel 和 Capability Adapter 的位置。
- 无状态 Adapter 可以直接注册构造函数；资源型 Adapter 由 Bootstrap 桥接配置、Reload 和 Close。
- 构造或配置失败必须向上返回，不能静默切换到 Noop 或第二套实现。

Kernel 必有的标准库 Slog 与 Dig、Koanf、fsnotify 位于
[`internal/adapter/kernel`](../../internal/adapter/kernel/README.md)，只实现内部 Kernel 协议，不属于
本使用中心。开发新 Adapter 的流程见
[Capability 与 Adapter](../../docs/development/capability-adapters.md)，权威依赖禁令见
[Adapter 与依赖边界](../../docs/maintenance/adapter-boundaries.md)。
