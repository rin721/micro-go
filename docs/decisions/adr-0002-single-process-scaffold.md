# ADR-0002：定位为单进程 Go 应用脚手架

状态：Accepted。

项目不再把 Kernel 作为外部框架 API。唯一入口是 `cmd/app`，唯一组合根是
`internal/bootstrap`；配置、生命周期、Reload、DI 和诊断协议收回 `internal/kernel`。

业务可复用能力契约放在 `types/capability`，具体实现放在 `pkg/adapter`。Capability Adapter
不得依赖 Kernel；组合根使用私有桥接完成生命周期和重载接入。默认 Kernel 实现位于
`pkg/adapter/kernel`，但不作为外部 SDK 兼容面。

本次采用单轨迁移，不保留旧路径或兼容别名。当前没有满足跨包、无状态、无资源且没有
明确所有者条件的工具，因此不建立 `pkg/utils`；避免该目录退化成无语义杂物包。
