# internal/adapter/kernel/di/compiled

本包定义声明通过 Compiler 校验后的冻结执行计划。

`Provider` 保存反射构造函数、所有权、稳定顺序和已解析依赖；`Binding` 表示接口别名；`Config` 表示模块配置；`Instance` 把构造结果与诊断元数据关联。

这些结构包含 `reflect.Type`、`reflect.Value` 和构造函数，因此只供 Kernel Adapter 内部协作。只读依赖图会被转换为 [`internal/kernel/di.Graph`](../../../../../internal/kernel/di/README.md)。

计划的生产者是 [`compiler`](../compiler/README.md)，消费者是 [`dig`](../dig/README.md) 和 Runtime。
