# 契约速查

脚手架不是外部框架 SDK。只有 `types/capability` 用于当前仓库业务包之间的稳定能力协作；
其余条目都是同一应用内部协议。

## Capability

| 契约 | 责任 | 当前实现 |
| --- | --- | --- |
| `logging.Logger` | 结构化日志、派生字段和命名空间 | Slog、Zap、Noop |
| `clock.Clock` | 返回当前时间 | System Clock |
| `idgen.Generator` | 返回字符串 ID | Google UUID |

## Module 与 DI

| 符号 | 精确语义 |
| --- | --- |
| `Module` | 提供唯一名称并在启动期登记声明 |
| `Registry` | 仅在 Register 期间写入 Provider、Binding、Export 和 Config |
| `Provide` | 登记返回具体类型的普通构造函数 |
| `Bind` | 将本模块具体实现映射为接口别名 |
| `Export` | 允许其他模块依赖本模块接口 |
| `Config` | 声明本模块拥有的强类型配置路径 |
| `Plan.DependencyGraph` | 返回可复制的只读节点和边，不提供实例解析 |

## 配置与 Reload

| 符号 | 精确语义 |
| --- | --- |
| `Source` | 读取一份 Map、JSON 或 YAML 配置事实 |
| `Snapshot` | 已验证配置的不可变版本事实 |
| `Value[T]` | 从 Snapshot 返回配置值深复制 |
| `Validator` | 配置类型的领域校验接口 |
| `Reloader` | 处理已验证候选并返回应用决定 |
| `Applied` | 已原地应用候选 |
| `Ignored` | 确认无需处理 |
| `RestartRequired` | 不能安全原地更新，应清理并退出 |

## 生命周期

`Preparer`、`Starter`、`Runner`、`Stopper`、`Closer`都是可选小接口，方法统一接收 Context。
正序阶段是 Prepare、Start；Runner 并发监督；逆序阶段是 Stop、Close。

## Application 与诊断

Application 状态依次覆盖 Registering、Compiling、Constructing、Built、Preparing、Starting、
Running、Stopping、Closing，并以 Closed、Failed 或 RestartRequired 结束。`ComponentError`
补充 Module、Component、Provider 和 Phase，`PanicError`保存 panic 值与堆栈；两者保留标准错误链。

符号定义分别位于 [`types/capability`](../../types/capability/README.md)和
[`internal/kernel`](../../internal/kernel/README.md)。
