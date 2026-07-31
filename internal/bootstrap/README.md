# internal/bootstrap

本包是唯一组合根，负责决定默认 Adapter、配置来源优先级、Module 列表和业务 Runner。

`Run(ctx)` 显式创建 Collector、Compiler、Koanf Loader、Dig Constructor、fsnotify Watcher，
再构造 Runtime。Slog 的关闭与 Reload 由私有 `managedLogger` 桥接，Slog Adapter 本身不需要
导入 Kernel 协议。

默认配置顺序是代码默认值、配置文件、`APP_` 环境变量。修改技术选型应集中在本包，业务
组件继续依赖 `types/capability`。`bootstrap_test.go` 使用真实默认栈验证启动和取消关闭。
