# micro-go

`micro-go` 是面向单进程 Go 应用的可复制脚手架。复制仓库后，业务直接在同一 Module
内开发；Kernel、默认技术栈和组合根都是应用内部实现，不是供其他 Module 导入的框架 SDK。

它负责启动期静态依赖图、强类型配置、组件生命周期、Runner 监督和失败退出式 Reload。
它不是微服务治理平台，也不预装 HTTP、数据库、缓存、消息或分布式能力；这些能力只有在
真实应用需要时，才通过项目契约、Adapter 和唯一组合根接入。

当前仓库仍是早期架构基线。默认 `process` 只记录启动信息并等待根 Context 取消，用于证明
完整运行链可工作，不代表已经存在业务系统或生产服务。

## 运行

```powershell
go run ./cmd/app
```

默认配置位于 [`config/app.yaml`](config/app.yaml)。环境变量使用 `APP_` 前缀，双下划线
表示层级，例如 `APP_LOGGING__LEVEL=debug`；`APP_CONFIG_FILE` 可以替换配置文件路径。
进程收到 Ctrl+C 或 SIGTERM 后会取消根 Context，再按依赖逆序停止和释放组件。

## 开发入口

- [`cmd/app`](cmd/app/README.md)：唯一进程入口，只处理信号、退出码和最终错误。
- [`internal/bootstrap`](internal/bootstrap/README.md)：唯一组合根，选择 Adapter、配置来源和模块。
- [`types/capability`](types/capability/README.md)：业务组件可以依赖的日志、时钟和 ID 契约。
- [`internal/kernel`](internal/kernel/README.md)：配置、DI、生命周期、Reload 和诊断协议。
- [`internal/adapter`](internal/adapter/README.md)：Kernel 协议的默认内部实现。
- [`pkg/adapter`](pkg/adapter/README.md)：Logger、Clock 和 ID 等 Capability 实现。

业务组件通过普通构造函数显式接收能力接口，不查询容器，也不直接创建 Zap、UUID、
Koanf 或 Dig 对象。替换技术栈时修改组合根和对应 Adapter，不修改消费者签名。

## 文档与验证

从 [文档中心](docs/README.md) 按“运行 → 开发 → 架构 → 当前状态 → 演进方向”阅读。每个
Go 包目录均保留 README，解释职责、边界和设计原因。

```powershell
./scripts/verify.ps1
```
