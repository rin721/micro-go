# 生命周期与 Reload

组件只实现自己真正拥有的阶段接口。所有阶段共享 Context 协作式预算；Runtime 不通过遗留
goroutine 伪造硬超时，因此阻塞 I/O 必须主动响应取消。

## 生命周期选择

| 接口 | 适用责任 | 顺序与所有权 |
| --- | --- | --- |
| `Preparer` | 建立可失败的外部资源准备 | 依赖正序；失败后关闭已构造组件 |
| `Starter` | 启动服务或后台能力 | 依赖正序；只有成功者会收到 Stop |
| `Runner` | 由 Application 监督的长期任务 | 全部 Start 成功后并发运行 |
| `Stopper` | 停止已经成功启动的活动 | 依赖逆序，消费者先停止 |
| `Closer` | 释放构造后拥有的资源 | 所有已构造组件都按逆序关闭 |

Runner 返回 error、panic 或意外正常返回都会结束 Application。生命周期、Observer 和清理错误
会聚合返回，不允许记录错误后伪装成功。

## Reload 决策

文件事件只触发候选重建。Runtime 从空配置树加载并验证完整候选，再通知直接依赖变化配置的
组件：

- `Applied`：组件已应用候选；
- `Ignored`：组件确认无需动作；
- `RestartRequired`：不能安全原地更新，应用完成清理后退出；
- error 或 panic：应用进入失败关闭。

无效候选不会替换当前 Snapshot，应用继续运行。若某些组件已应用后后续组件失败，Runtime
不会承诺跨组件回滚，而是立即失败退出，避免长期运行在混合状态。完整决策见
[ADR-0003](../decisions/adr-0003-reload-failure-exit.md)。

实现 Reloader 时必须同步保护与 Runner 共享的状态，并尊重同一次 Reload 的 Context 预算。
当前 Slog 桥接只允许级别原地更新；输出位置或编码器变化会请求重启。
