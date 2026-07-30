# internal/di/constructor

本包定义 Application 使用的内部构造 `Engine` 接口。

接口只接受 Context、已编译 Plan 和强类型配置值，并返回项目 `Instance`。因此 Application 不需要导入 Dig，未来更换执行引擎也不会修改公共 API。

Engine 必须遵守项目已计算的顺序，并在失败时负责关闭已构造资源；不能在运行期保留容器或提供 Resolve。当前实现是 [`digadapter`](../digadapter/README.md)。

