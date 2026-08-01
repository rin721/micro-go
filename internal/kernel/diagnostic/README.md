# internal/kernel/diagnostic

本包定义跨注册、配置、构造和生命周期阶段共享的稳定错误模型。

## 为什么这样设计

第三方类型不得进入项目契约签名，但错误原因必须保留。`ComponentError` 使用项目 `Phase`
补充模块、组件和 Provider 上下文，同时通过 `Unwrap` 保留 Cause。`PanicError` 保存 panic 值
与堆栈，使失败仍能进入正常回滚路径。

## 使用边界

- Provider 和生命周期业务错误保留在 Cause 中。
- Dig、validator、Koanf 等错误由内部 Adapter 补充稳定操作上下文，并保留原始错误链；业务
  不应根据第三方错误文本决定策略。
- Observer 看到的是项目 Event 和项目错误，不会获得容器对象。

错误产生与聚合过程可从 [`internal/adapter/kernel/runtime`](../../../internal/adapter/kernel/runtime/README.md)开始阅读。
