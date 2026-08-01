# Internal 设计入口

`internal` 保存当前应用自己的装配、Kernel 协议和架构门禁，Go 的 internal 规则阻止其他
Module 把这些包当成公共框架 API。

- [bootstrap](bootstrap/README.md)：唯一组合根。
- [kernel](kernel/README.md)：配置、DI、生命周期、Reload 和诊断协议。
- [adapter](adapter/README.md)：Kernel 协议的默认内部实现。
- [architecture](architecture/README.md)：依赖方向与第三方污染检查。

业务 Capability 实现位于 [`pkg/adapter`](../pkg/adapter/README.md)，Kernel 默认实现位于
[`internal/adapter`](adapter/README.md)。依赖方向始终是实现指向协议，`internal/kernel`
不得反向导入 Adapter。

跨目录关系由[架构概念](../docs/concepts/architecture.md)统一说明，本页只导航内部源码。
