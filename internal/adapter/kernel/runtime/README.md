# internal/adapter/kernel/runtime

本包是默认 Kernel 编排实现：收集模块、加载配置、编译依赖图、事务构造组件、驱动生命周期、监督 Runner、处理 Reload 并统一关闭。

## 为什么这样设计

Application 是资源所有权和状态转换的唯一协调者。DI 图决定构造及生命周期顺序，避免配置、容器和生命周期各自维护一套互相冲突的依赖关系。实例构造完成后不保留可供业务查询的容器。

## 运行链

1. `Runtime.Compile` 应用 Options、收集并冻结 Registry、校验图、加载配置。
2. `Runtime.Build` 按计划构造组件；失败时逆序 Close 已构造实例。
3. `Run` 正序 Prepare/Start，再并发监督 Runner。
4. 文件变化经去抖后全量生成候选 Snapshot，只通知受影响组件。
5. 退出时取消 Runner、逆序 Stop、等待 Runner、逆序 Close，并聚合错误。

## 关键不变量

- Application 只能 Run 一次。
- Stop 只调用成功 Start 的组件，Close 覆盖全部构造成功的组件。
- Startup、Shutdown 和 Reload 使用各自的共享 Context 预算；调用必须协作响应取消。
- 候选配置只有在所有组件接受后才被提升。
- Observer 同步、只读并受 panic 保护；事件 panic 会进入错误聚合且不能跳过清理。
- 未配置监听时不创建 Runtime goroutine。

Collector、Compiler、Loader、Constructor 和 Watcher 都由 Bootstrap 显式注入。本包不自行选择 Dig、Koanf 或 fsnotify，因此技术栈替换不需要改动状态机。

## 验证

[`app_test.go`](app_test.go)覆盖图与构造回滚，
[`lifecycle_failure_test.go`](lifecycle_failure_test.go)覆盖生命周期故障，
[`reload_test.go`](reload_test.go)覆盖候选提交边界，`main_test.go` 执行 goroutine 泄漏检查。
