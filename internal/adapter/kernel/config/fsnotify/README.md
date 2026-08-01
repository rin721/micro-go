# internal/adapter/kernel/config/fsnotify

本包将 fsnotify 文件事件转换为项目 `config.Change`，不向上暴露 `fsnotify.Event`。

## 设计原因

编辑器常通过临时文件和 rename 保存，因此监听父目录再筛选目标路径，比只绑定文件更可靠。事件和错误通道容量为一并采用非阻塞发送；重复事件可以丢弃，因为任一通知都会触发 Application 全量重建候选。

本包不负责去抖、合并配置或提升 Snapshot。goroutine 生命周期完全绑定调用方 Context，未启用 Watch 时 Application 不会调用它。

上层流程见 [`runtime`](../../runtime/README.md)。
