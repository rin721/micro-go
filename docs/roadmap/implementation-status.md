# 实现状态

## 已验证能力

| 能力 | 证据 |
| --- | --- |
| 唯一 `cmd/app` 入口和 Bootstrap | [`boundary_test.go`](../../internal/architecture/boundary_test.go)、[`bootstrap_test.go`](../../internal/bootstrap/bootstrap_test.go) |
| Registry 冻结和声明错误隔离 | [`collector_test.go`](../../internal/adapter/kernel/module/collector_test.go) |
| Provider/模块循环、可见性和稳定图 | [`compiler_test.go`](../../internal/adapter/kernel/di/compiler/compiler_test.go)、[`app_test.go`](../../internal/adapter/kernel/runtime/app_test.go) |
| 事务构造、Provider panic 和逆序 Close | [`constructor_test.go`](../../internal/adapter/kernel/di/dig/constructor_test.go)、[`app_test.go`](../../internal/adapter/kernel/runtime/app_test.go) |
| 强类型配置、未知字段拒绝、优先级、校验和深复制 | [`loader_test.go`](../../internal/adapter/kernel/config/koanf/loader_test.go)、[`source_test.go`](../../internal/adapter/kernel/config/source/source_test.go) |
| 生命周期、默认 Observer、超时和错误聚合 | [`lifecycle_failure_test.go`](../../internal/adapter/kernel/runtime/lifecycle_failure_test.go)、[`observer_test.go`](../../internal/bootstrap/observer_test.go) |
| 启动窗口配置重读、文件监听和失败退出式 Reload | [`watcher_test.go`](../../internal/adapter/kernel/config/fsnotify/watcher_test.go)、[`app_test.go`](../../internal/adapter/kernel/runtime/app_test.go)、[`reload_test.go`](../../internal/adapter/kernel/runtime/reload_test.go) |
| Slog/Zap 可替换日志契约 | [`contract_test.go`](../../pkg/adapter/logging/contract_test.go) |
| Work Item HTTP/SQLite 验收纵切片 | [`backend_acceptance_test.go`](../../internal/bootstrap/backend_acceptance_test.go)、[`backend_process_unix_test.go`](../../internal/bootstrap/backend_process_unix_test.go) |
| 依赖方向、文档与 README 门禁 | [`internal/architecture`](../../internal/architecture/README.md) |

## 已知限制

- 生命周期超时依赖组件遵守 Context，不是可强制抢占的硬超时。
- Reload 没有跨组件回滚；部分应用后失败会关闭应用，不提升候选 Snapshot。
- 没有实例代际、局部图重建、命名或集合注入和多作用域。
- 默认 `process`只是运行链证明；Work Item HTTP/SQLite 实现仅由 `integration`验收测试装配，
  尚未成为默认业务产品。
- GitHub Actions run `30683789518`已实证 Windows/Linux unit、integration 与项目初始化，以及
  Linux Shell 初始化、Unix SIGTERM 和 scratch 非 root 容器退出码，跨平台门禁已闭环。

## 下一阶段

优先继续补充真实故障注入、错误诊断和具体应用需要的 Capability。HTTP、数据库、缓存、
消息和可观测性只有在出现明确调用方与验收条件后才进入实现，不作为默认栈预建。准入规则
见[演进方向](evolution.md)。
