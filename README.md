# micro-go

`micro-go` 是面向单进程 Go 应用的基础设施依赖注入与运行治理框架。它在构建阶段完成配置、模块边界、依赖图和生命周期计划，在运行阶段只保留普通 Go 接口调用。

## 快速运行

```powershell
go run ./examples/basic
```

示例显式启用 Slog、系统时钟、UUID 和应用组件模块。切换日志实现只需在应用组装根把 `slog.Module{}` 换成 `zap.Module{}`，消费者仍依赖 `capability/logging.Logger`。

## 核心入口

- `kernel/app`：Compile、Build、Run、状态和 Observer。
- `kernel/module`：Module、Provide、Bind、Export、Config。
- `kernel/config`：配置源与不可变 Snapshot。
- `kernel/lifecycle`：Prepare、Start、Run、Stop、Close。
- `capability`：稳定公共能力契约。
- `adapter`：Dig、Koanf 之外的具体能力实现选择；第三方对象不会进入公共契约。

完整阅读路线见 [docs/README.md](docs/README.md)，当前实现的就地设计说明见下方源码设计地图。

## 源码设计地图

需要快速理解“代码在哪里、为什么这样分层”时，按下面的入口就地阅读：

- [Kernel](kernel/README.md)：公共框架契约、依赖图、配置、生命周期和 Application。
- [Capability](capability/README.md)：业务可长期依赖的日志、时钟和 ID 小接口。
- [Adapter](adapter/README.md)：Slog、Zap、System Clock 和 UUID 的具体实现与隔离方式。
- [Internal](internal/README.md)：注册、Compiler、Dig、Koanf 和 fsnotify 的内部执行链。
- [Examples](examples/README.md)：只使用公共契约完成真实应用组装。

每个包含 Go 源码的包目录都保留独立 `README.md`。顶层入口只维护导航，包级 README 负责说明职责、非职责、运行流程、边界和验证方式，源码仍是行为事实来源。

## 验证

```powershell
./scripts/verify.ps1
```
