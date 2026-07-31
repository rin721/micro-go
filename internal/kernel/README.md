# Internal Kernel 设计入口

`internal/kernel` 拥有单进程运行时协议和值模型，不选择第三方技术栈。

- [app](app/README.md)：状态、事件、Observer、Plan 与 Application 契约。
- [module](module/README.md)：注册语言。
- [config](config/README.md)：Source、Snapshot 与校验模型。
- [di](di/README.md)：只读依赖图。
- [lifecycle](lifecycle/README.md)：组件生命周期小接口。
- [reload](reload/README.md)：候选配置应用结果。
- [diagnostic](diagnostic/README.md)：阶段化错误模型。
- [testkit](testkit/README.md)：内部测试辅助。

这里禁止导入第三方库和 `pkg/adapter`。默认实现从
[`pkg/adapter/kernel`](../../pkg/adapter/kernel/README.md)进入。
