# Capability 与 Adapter

Capability 表达业务需要的稳定能力，Adapter 把标准库或成熟第三方实现收敛到该契约。只有
出现真实调用方和技术边界时才新增能力。

## 接入流程

1. 从消费者需要定义最小接口，放入有业务语义的 `types/capability/<name>` 包。
2. 评估标准库、现有实现和成熟第三方库，不为通用能力重复造轮子。
3. 在 `pkg/adapter/<name>/<implementation>` 实现契约，只暴露项目类型。
4. 在实现模块中 `Provide` 具体类型，`Bind` 到 Capability，并按需 `Export`。
5. 由 Bootstrap 选择唯一实现；消费者构造函数只接收 Capability。
6. 用共享契约测试验证可替换行为，再覆盖资源、错误、取消和配置变化。

当前实例包括 [`logging`](../../types/capability/logging/README.md)、
[`clock`](../../types/capability/clock/README.md)和
[`idgen`](../../types/capability/idgen/README.md)。它们的实现位于
[`pkg/adapter`](../../pkg/adapter/README.md)。

## 资源型 Adapter

Capability Adapter 不导入 Kernel。若实现需要配置、Reload 或 Close，Bootstrap 使用私有桥接
把 Adapter 自有结果转换为 Kernel 契约；当前 `managedLogger` 就是该模式。调用方不得绕过
桥接自行创建第二个客户端或连接。

Kernel 自身的 Dig、Koanf 和 fsnotify 实现属于内部执行机制，放在
[`internal/adapter/kernel`](../../internal/adapter/kernel/README.md)，不能作为业务 Capability。
完整依赖禁令见[Adapter 与依赖边界](../maintenance/adapter-boundaries.md)。
