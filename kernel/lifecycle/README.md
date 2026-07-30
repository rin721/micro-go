# kernel/lifecycle

本包把组件生命周期拆成五个可选小接口：`Preparer`、`Starter`、`Runner`、`Stopper`、`Closer`。

## 为什么这样设计

组件只实现自己需要的能力，不继承庞大的基类或第三方 Lifecycle。接口拆分还能精确表达所有权：只有 Start 成功的组件需要 Stop，但所有构造成功且实现 Closer 的组件都必须 Close。

## 顺序

- Prepare、Start：依赖正序。
- Run：全部 Start 成功后并发监督。
- Stop、Close：依赖逆序，消费者先退出。
- Runner 返回错误或意外正常返回都会触发统一关停。

调度实现位于 [`kernel/app/run.go`](../app/run.go)，失败与回滚场景位于 [`kernel/app/app_test.go`](../app/app_test.go)。

