# 架构与运行链

脚手架固定执行：模块注册、Registry 冻结、依赖图编译、配置加载校验、事务构造、Prepare、
Start、Runner 监督、Stop、Close。Graph Compiler 决定模块可见性、唯一绑定、Provider 环、
模块环和稳定拓扑顺序；Dig 只执行已验证计划，Koanf 只构建候选配置树。

`types/capability` 是业务依赖方向的终点，`pkg/adapter` 实现这些业务能力；
`internal/kernel` 拥有运行协议，`internal/adapter/kernel` 提供默认执行实现。
`internal/bootstrap` 是唯一同时选择两类 Adapter 的组合根。Application 进入 Running 后
不保留可供业务查询的容器。

构造、Prepare 和 Start 失败会按已完成阶段逆序补偿。Runner 共享可取消 Context，关闭时
先停止活动，再等待后台任务退出，最后释放资源；生命周期、Observer 和清理错误通过
`errors.Join` 保留。所有超时都是 Context 协作式预算，组件必须主动响应取消，Runtime 不会
遗留无法回收的强制中断 goroutine。
