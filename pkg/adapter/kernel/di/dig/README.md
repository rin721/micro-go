# pkg/adapter/kernel/di/dig

本包把已编译 Plan 转换为临时 Dig 容器并事务性构造实例。

## Dig 的限定职责

- 注册强类型配置和普通 Provider。
- 为 Binding 创建返回同一实例的接口别名 Provider。
- 按项目拓扑顺序逐个 Invoke，成功一个就登记一个。
- 捕获 Provider panic，并把 Dig 错误归一化为项目错误。

## 事务和所有权

任一步失败都会按构造逆序调用已登记 Closer，并使用 `errors.Join` 保留根因和清理错误。成功后只返回普通实例，Dig 容器不进入 Application 运行期。

项目 Compiler 已提前处理可见性、唯一性和循环；这里不允许 Dig 重新定义架构规则，也不支持 `dig.In`、`dig.Out`、Name、Group 或 Scope。
