# kernel/di

本包提供与具体容器无关的依赖图值模型，以及 Text、DOT、JSON 三种导出方式。

## 为什么这样设计

依赖图是诊断结果，不是运行期解析入口。公开普通值可以支持可视化和审计，同时避免调用方获得 Dig 容器、反射构造函数或实例引用。

## 模型

- `Node` 表示 Provider 或强类型配置，`Order` 是稳定拓扑顺序。
- `Edge` 从依赖指向消费者，`Via` 记录消费者请求的原始类型。
- `Graph` 只包含可复制、可序列化的数据。

稳定排序和模块可见性由 [`internal/di/compiler`](../../internal/di/compiler/README.md)计算。应用通过 `Plan.DependencyGraph` 返回切片副本，外部修改不会影响编译计划。

## 验证

依赖图顺序和生命周期标记由 [`kernel/app/app_test.go`](../app/app_test.go)覆盖。

