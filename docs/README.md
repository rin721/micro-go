# 文档中心

建议按以下顺序阅读：

1. [创建第一个应用](getting-started/first-application.md)
2. [开发模块与 Provider](development/modules-and-providers.md)
3. [架构与运行链](concepts/architecture.md)
4. [公共 API](reference/api.md)
5. [依赖与配置适配器边界](internals/adapters.md)
6. [实现状态](roadmap/implementation-status.md)

## 从源码设计阅读

如果目标是快速理解实现和设计原因，可以从以下就地文档进入：

- [Kernel 公共契约](../kernel/README.md)
- [Capability 稳定能力](../capability/README.md)
- [Adapter 具体实现](../adapter/README.md)
- [Internal 编译与执行引擎](../internal/README.md)
- [Examples 组合根](../examples/README.md)

每个 Go 包的 README 会继续链接核心源码和测试；概念文档解释整体架构，包级 README 解释局部实现，两者不重复维护相同细节。

框架当前只支持 Application Singleton，不提供运行时容器查询、字段注入、自动扫描和动态代理。
