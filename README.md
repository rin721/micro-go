# micro-go

`micro-go` 是面向单进程 Go 应用的可复制脚手架。复制仓库后，应用直接在同一 Module
内开发；Kernel、默认技术栈和组合根属于项目内部实现，不再作为通用框架 API 对外承诺。

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
- [`pkg/adapter`](pkg/adapter/README.md)：Capability 与 Kernel 协议的具体实现。

业务组件通过普通构造函数显式接收能力接口，不查询容器，也不直接创建 Zap、UUID、
Koanf 或 Dig 对象。替换技术栈时修改组合根和对应 Adapter，不修改消费者签名。

## 文档与验证

从 [文档中心](docs/README.md) 按“运行 → 开发 → 架构 → 内部实现”阅读。每个 Go 包目录
均保留 README，解释职责、边界和设计原因。

```powershell
./scripts/verify.ps1
```
