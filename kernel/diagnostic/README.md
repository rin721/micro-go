# kernel/diagnostic

本包定义跨注册、配置、构造和生命周期阶段共享的稳定错误模型。

## 为什么这样设计

第三方库错误如果直接外泄，调用方就会依赖实现细节。`ComponentError` 使用项目 `Phase` 补充模块、组件和 Provider 上下文，同时通过 `Unwrap` 保留业务 Cause。`PanicError` 保存 panic 值与堆栈，使失败仍能进入正常回滚路径。

## 使用边界

- Provider 和生命周期业务错误保留在 Cause 中。
- Dig、validator、Koanf 等错误先由内部 Adapter 归一化。
- Observer 看到的是项目 Event 和项目错误，不会获得容器对象。

错误产生与聚合过程可从 [`kernel/app`](../app/README.md)开始阅读。

