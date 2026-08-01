# ADR-0002：定位为单进程 Go 应用脚手架

状态：Accepted

## 背景

Kernel 曾可能被理解为供其他 Module 导入的框架接口，这会要求维护外部兼容面，并诱导 Runtime
Resolve、自动扫描和双轨目录继续增长。当前真实用途是在一个仓库内装配单个 Go 进程。

## 决策

项目不再把 Kernel 作为外部框架 API。唯一入口是 `cmd/app`，唯一组合根是
`internal/bootstrap`；配置、生命周期、Reload、DI 和诊断协议收回 `internal/kernel`。

业务可复用能力契约放在 `types/capability`，具体实现放在 `pkg/adapter`。Capability Adapter
不得依赖 Kernel；组合根使用私有桥接完成生命周期和重载接入。默认 Kernel 实现物理放在
`internal/adapter/kernel`，由 Go internal 规则阻止外部 Module 导入。

本次采用单轨迁移，不保留旧路径或兼容别名。当前没有满足跨包、无状态、无资源且没有
明确所有者条件的工具，因此不建立 `pkg/utils`；避免该目录退化成无语义杂物包。

## 后果

- 业务直接在复制后的同一 Go Module 内开发，不承诺 Kernel 的跨项目兼容。
- Bootstrap 集中选择模块、配置来源、Adapter 和运行时参数。
- 不发展外部 SDK、运行期 Resolve、自动扫描或动态插件系统。
- 新能力由真实应用需求推动，通过 Capability、Adapter 和 Bootstrap 接入。
