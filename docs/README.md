# 文档中心

建议按以下顺序阅读：

1. [运行脚手架](getting-started/first-application.md)
2. [开发 Module 与 Provider](development/modules-and-providers.md)
3. [架构与运行链](concepts/architecture.md)
4. [仓库内部契约](reference/api.md)
5. [第三方适配边界](internals/adapters.md)
6. [实现状态](roadmap/implementation-status.md)

## 源码设计入口

- [进程入口](../cmd/README.md)
- [唯一组合根与内部 Kernel](../internal/README.md)
- [公共能力类型](../types/README.md)
- [具体实现包](../pkg/README.md)

概念文档解释整体关系，包级 README 解释局部职责，源码与测试是行为事实来源。当前项目
只治理一个进程内的 Application Singleton，不提供运行时 Resolve、自动扫描、多作用域、
动态代理、业务事件总线或分布式治理。
