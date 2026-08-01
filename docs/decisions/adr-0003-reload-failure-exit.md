# ADR-0003：Reload 采用失败退出模型

状态：Accepted

## 背景

当前组件只实现单阶段 `Reloader`，没有跨组件 Prepare/Commit/Rollback 协议或实例代际。把部分
应用后的失败继续描述为可恢复，会让 Application 在新旧配置混合的状态下运行。

## 决策

当前 Runtime 每次从空配置树生成并验证完整候选 Snapshot，只通知直接依赖变化配置的组件。
所有受影响组件接受后才提升 Snapshot；无效候选保留旧版本并继续运行。

`Reloader` 可以返回 Applied、Ignored 或 RestartRequired。组件不支持原地更新、返回
RestartRequired，或者部分组件应用后发生错误时，Application 进入统一关闭路径。当前契约
不提供跨组件 Rollback、两阶段提交、实例代际或局部图重建，因此不得宣称原子重载。

候选加载和 Reloader 调用共享一次 Context 超时。该超时依赖实现主动响应取消；Runtime 不会
通过遗留 goroutine 强制抢占调用。

## 后果

- 无效候选不影响当前运行实例。
- RestartRequired 在清理成功后以明确最终状态退出。
- 部分组件已经应用后发生 error、panic 或超时，Application 进入 Failed 并统一关闭。
- 当前不承诺原子 Reload、跨组件回滚、局部图重建或实例代际。
- 只有出现量化的连续可用性目标时，才能用新 ADR 选择两阶段提交或代际替换，并在同一次
  迁移中删除本模型。
