# internal/di/compiler

本包是项目架构规则的权威实现，把注册 Collection 编译为稳定 Plan。

## 编译规则

- Provider 必须是非可变参数普通函数，返回 Concrete 或 `(Concrete, error)`。
- Provider、配置类型和接口 Binding 必须唯一。
- Binding 只能引用本模块 Provider，Export 只能公开本模块接口 Binding。
- 配置只能由所属模块使用，跨模块依赖必须通过已导出接口。
- Context、Registry 和容器标记类型不能成为 Provider 依赖。
- 缺失依赖、循环和私有具体类型越界在 Build 前失败。

## 稳定顺序

Compiler 使用 Kahn 拓扑排序，并以模块声明顺序和模块内注册顺序处理多个就绪节点。这样构造、生命周期、图输出和错误顺序均可复现，不能交给 Dig 的未指定实例化顺序。

成功结果位于 [`compiled`](../compiled/README.md)，执行见 [`digadapter`](../digadapter/README.md)。

