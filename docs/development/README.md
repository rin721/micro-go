# 应用开发路线

这条路线面向在当前 Go Module 内实现业务组件的开发者。所有步骤都以现有
[`application.process`](../../internal/bootstrap/module_application.go)为事实样本。

1. [开始运行](../getting-started/README.md)，确认默认进程和配置可用。
2. [组件接入工作流](component-workflow.md)，理解构造函数、Module 和 Bootstrap。
3. [Capability 封装与注入](capability-adapters.md)，沿当前 Clock、UUID 和 Logging 的真实代码
   学习从契约、Adapter、Options 到消费者字段的完整路径。
4. [配置开发](configuration.md)，为模块声明并注入强类型配置。
5. [生命周期与 Reload](lifecycle-and-reload.md)，选择资源拥有方式和失败策略。
6. [验证与故障定位](../maintenance/verification.md)，补齐成功、失败和并发证据。

业务代码不得查询容器、保存 Registry、导入 `internal/adapter/kernel`，也不得绕过组合根创建
第二套共享客户端。精确契约名称见[内部契约索引](../reference/README.md)。
