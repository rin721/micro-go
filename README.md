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

完整阅读路线见 [docs/README.md](docs/README.md)，原始目标设计见 [design.md](design.md)。

## 验证

```powershell
./scripts/verify.ps1
```

