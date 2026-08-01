# Kernel 配置 Adapter

本目录把 `internal/kernel/config` 协议连接到具体来源和第三方执行引擎。

- [source](source/README.md)：map、文件、环境变量和 Flag 来源。
- [koanf](koanf/README.md)：严格合并、解码、校验与候选 Snapshot。
- [fsnotify](fsnotify/README.md)：文件变化通知。

三者分别负责读取、构建候选和通知变化；Watcher 不直接调用业务组件，Koanf Loader 也不
持有当前 Snapshot。当前版本只能由 Runtime 提升。
