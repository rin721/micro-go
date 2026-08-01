# Runtime 执行链

默认 Runtime 是状态与资源所有权的协调者。它依赖 Bootstrap 注入的 Collector、Compiler、
Loader、Constructor 和 Watcher，不自行选择 Dig、Koanf 或 fsnotify。

## Compile 与 Build

1. 应用 Options，拒绝 nil Option 和非法超时。
2. Observer 收到 Registering，Collector 执行所有 `Module.Register` 并冻结 Registry。
3. Compiler 校验声明、跨模块可见性、Provider 图和模块图，生成稳定 Plan。
4. Loader 合并并验证初始配置，生成版本 1 Snapshot。
5. Dig Constructor 按拓扑顺序构造实例；失败时逆序 Close 已构造实例。
6. 构造全部成功后发布 Built Application，Dig 容器随即退出运行边界。

Compile 只返回只读依赖图，不构造组件；Build 才产生需要 Application 管理的实例。

## Run 与关闭

Application 只能 Run 一次。Prepare 和 Start 共享启动预算，成功 Start 的组件被记录；随后所有
Runner 使用同一个可取消 Context 并发运行。任一 Runner error、panic 或意外返回都会进入统一
关闭路径。

关闭顺序是：取消 Runner → 状态 Stopping → 逆序 Stop 已启动组件 → 等待 Runner → 状态
Closing → 逆序 Close 全部已构造组件 → 发布最终状态。Stop、Close、Runner 和 Observer 的
错误全部聚合，不能让后发生的清理错误覆盖首要原因。

## 最终状态

| 条件 | 最终状态 |
| --- | --- |
| 根 Context 取消且清理成功 | `Closed` |
| Reload 仅要求重启且清理成功 | `RestartRequired` |
| 运行、Reload、Observer 或清理失败 | `Failed` |

## Reload

文件事件经 Runtime 去抖后共享一次 Reload Context 预算：Loader 生成完整候选，Runtime 根据
配置摘要找出受影响组件，再按稳定实例顺序调用 Reloader。无效候选只产生
`ConfigurationFail`并保留旧 Snapshot；全部组件接受后才提升版本。部分应用后失败立即关闭，
不承诺跨组件回滚。

权威实现位于 [`compile.go`](../../internal/adapter/kernel/runtime/compile.go)和
[`run.go`](../../internal/adapter/kernel/runtime/run.go)，故障矩阵位于同包测试。
