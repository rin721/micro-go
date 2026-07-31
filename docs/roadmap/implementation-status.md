# 实现状态

已实现：唯一 `cmd/app` 入口、唯一 Bootstrap、模块注册与冻结、稳定依赖图、事务 Build、
强类型配置、候选 Snapshot、文件监听、最小 Reload、完整生命周期、Runner、Observer、
结构化错误、Panic 边界、Slog/Zap、System Clock、UUID 以及第三方边界门禁。

明确未实现：运行时 Resolve、自动扫描、字段注入、多作用域、命名或集合注入、实例代际、
局部图重建、动态代理、业务事件总线、多进程协调和分布式治理。
