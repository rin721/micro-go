# micro-go

`micro-go` 是复制后在同一 Go Module 内继续开发的单进程应用脚手架。Kernel、默认技术栈和
组合根都是应用内部实现，不是供其他 Module 导入的框架 SDK。

当前仓库处于早期架构基线：已经实现启动期静态依赖图、强类型配置、组件生命周期、Runner
监督和失败退出式 Reload；默认 `process` 只记录启动信息并等待根 Context 取消，不代表已经
存在业务系统或生产服务。HTTP、数据库、缓存、消息等能力只在出现真实需求后接入。

仓库另有一套只在 `integration`门禁装配的
[Work Item 后端纵切片](docs/development/backend-acceptance.md)，用于证明 HTTP、SQLite、健康检查、
事务、重启持久化和优雅退出能沿同一 Module/DI/Runtime 链工作；它不是默认产品栈。

用于新项目时，先按[从模板创建后端项目](docs/getting-started/new-project.md)在独立副本中替换
Module path 与应用标识，并通过 fresh tests；不要只修改 `go.mod` 后留下旧 import。

## 快速运行

```powershell
go run ./cmd/app
```

默认配置位于 [`config/app.yaml`](config/app.yaml)。按 Ctrl+C 后，应用会取消 Runner，并按
依赖逆序停止和释放组件。配置覆盖、预期输出和常见失败见[开始运行](docs/getting-started/README.md)。

## 选择阅读路线

- [应用开发路线](docs/development/README.md)：运行现有进程，理解 `process`，再学习 Module、
  配置、生命周期和 Capability 接入。
- [Kernel 维护路线](docs/maintenance/README.md)：理解依赖图、Runtime、Adapter 边界、错误与
  Reload 语义，再进入源码维护。

[文档中心](docs/README.md)说明各类文档的唯一职责；包级 README 只解释相邻源码的局部边界。

## 完整验证

```powershell
./scripts/verify.ps1
```

该门禁先运行快速单元与静态检查，再运行带 `integration`标签的真实 HTTP/SQLite 测试及 race。
当前已实现
能力、已知限制和后续准入条件分别见[实现状态](docs/roadmap/implementation-status.md)与
[演进方向](docs/roadmap/evolution.md)。
