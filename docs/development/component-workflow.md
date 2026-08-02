# 组件接入工作流

本页沿当前 `process` 的真实装配链说明组件如何从普通 Go 类型进入 Application。

## 1. 从消费者需要的能力开始

[`process`](../../internal/bootstrap/module_application.go)只保存 `logging.Logger`、`clock.Clock` 和
`idgen.Generator`，没有容器、Registry 或第三方客户端。构造函数 `newProcess` 使用普通参数
显式表达依赖，因此依赖图可以在启动前编译。

新增业务组件时先定义它的业务职责和所需能力。只有跨业务包稳定协作的能力才进入
`types/capability`；组件自己的具体类型留在拥有它的业务包。新能力怎样从契约、Adapter 进入
构造参数和字段，统一见[Capability 封装与注入](capability-adapters.md)。

## 2. 使用普通构造函数

Provider 必须是非可变参数普通函数，返回一个具体类型，允许第二个返回值为 `error`。构造
阶段只创建内存对象；需要 Context、外部连接或可失败准备的动作放入生命周期方法。

不要在 Provider 内查询 Runtime，也不要把 `Registry`、`Application`、Dig 类型或配置引擎
类型作为参数。完整编译约束见 [`compiler`](../../internal/adapter/kernel/di/compiler/README.md)。

## 3. 在 Module 中声明

[`applicationModule.Register`](../../internal/bootstrap/module_application.go)通过 `module.Provide` 注册
`newProcess`。Module 只登记声明，不直接调用构造函数。模块名在一次编译中必须非空且唯一，
Register 返回后 Registry 会冻结。

模块向其他模块提供接口时，必须在同一 Module 内完成 `Provide`、`Bind` 和 `Export`；消费者
只接收导出的接口。跨模块直接依赖具体类型会在 Build 前失败；三项声明怎样形成同实例注入链，
见[Capability 封装与注入](capability-adapters.md)。

## 4. 只在 Bootstrap 组合

将 Module 加入 [`runtime.Build`](../../internal/bootstrap/bootstrap.go)的 `WithModules` 列表。
Bootstrap 是唯一同时选择 Kernel Adapter 和 Capability Adapter 的位置；业务包不维护第二个
组合根，也不通过自动扫描改变模块集合。

## 5. 选择生命周期

当前 `process` 只实现 `Runner`，因为它没有需要单独准备、启动或释放的资源。拥有资源的组件
按实际需要实现 `Preparer`、`Starter`、`Stopper` 或 `Closer`，不要为了形式实现空方法。方法
顺序和失败补偿见[生命周期与 Reload](lifecycle-and-reload.md)。

## 6. 验证接入

- 为构造函数和业务行为写同包测试。
- 为注册错误、缺失依赖、跨模块可见性和循环补充 Compiler 或 Runtime 测试。
- 为资源组件注入 Prepare、Start、Runner、Stop、Close 的 error、panic 和取消场景。
- 执行[完整门禁](../maintenance/verification.md)，确认依赖边界和 README 同步。
