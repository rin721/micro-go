# Adapter 与依赖边界

项目区分业务 Capability Adapter 与 Kernel Adapter，两者由 Bootstrap 在唯一组合根中汇合。

## Capability Adapter

`pkg/adapter` 实现 `types/capability` 中由消费者定义的能力。它可以使用标准库或第三方库，
但导出签名不得出现第三方类型，也不得导入 Kernel 配置、生命周期或 Reload 协议。资源关闭
和配置调整通过 Bootstrap 私有桥接接入 Runtime。

当前实现包括 System Clock、Google UUID、Slog、Zap 和 Noop Logger。业务组件只依赖 Clock、
Generator、Logger 和项目 Field，不依赖具体实现。

## Kernel Adapter

`internal/adapter/kernel` 实现应用内部的 Module 收集、图编译、配置加载、构造和 Runtime：

| 子系统 | 默认实现 | 不能拥有的责任 |
| --- | --- | --- |
| Module Collector | 自研 Registry | 解释 Provider 类型或构造实例 |
| Graph Compiler | 自研静态编译器 | 运行期 Resolve |
| Constructor | Dig | 决定可见性、循环和拓扑规则 |
| Config Loader | Koanf + validator | 持有当前 Snapshot |
| File Watcher | fsnotify | 调用业务 Reloader 或执行去抖 |
| Runtime | 项目状态机 | 自行选择上述具体实现 |

第三方错误在 Adapter 边界补充项目上下文并保留原因链；日志和错误摘要不得包含配置值或凭据。

## 自动门禁

[`internal/architecture`](../../internal/architecture/README.md)检查 `types`、Kernel、Adapter、
Bootstrap 与 `cmd/app` 的 import 方向，并拒绝 Adapter 导出第三方类型或恢复旧目录。新增例外
必须有明确架构理由和对应负例测试，不能扩大通配白名单。

新 Capability 的操作步骤见[Capability 与 Adapter](../development/capability-adapters.md)，第三方
边界的已确认决策见[ADR-0001](../decisions/adr-0001-third-party-boundary.md)。
