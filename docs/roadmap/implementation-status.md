# 实现状态

## 已验证能力

- 唯一 `cmd/app` 入口和唯一 Bootstrap。
- Registry 冻结、Provider 与模块循环检测、稳定依赖图和事务构造。
- 强类型配置、不可变候选 Snapshot、文件监听和失败退出式 Reload。
- Context 协作式 Prepare、Start、Runner、Stop、Close 与逆序资源释放。
- 结构化错误、Panic 边界、Observer 事件和多错误聚合。
- Slog/Zap、System Clock、UUID，以及第三方依赖方向门禁。

## 已知限制

- 生命周期超时依赖组件遵守 Context，不是可强制抢占的硬超时。
- Reload 没有跨组件回滚；部分应用后失败会关闭应用，不提升候选 Snapshot。
- 没有实例代际、局部图重建、命名或集合注入和多作用域。
- 默认 `process` 只是运行链证明，没有真实业务、传输协议或持久化能力。

## 下一阶段

优先继续补充真实故障注入、错误诊断和具体应用需要的 Capability。HTTP、数据库、缓存、
消息和可观测性只有在出现明确调用方与验收条件后才进入实现，不作为空泛默认栈预建。
长期边界见[演进方向](evolution.md)。
