# Kernel Adapter 设计入口

本目录提供 `internal/kernel` 协议的默认实现，但不是外部 SDK 兼容面。

- [module](module/README.md)：Registry 与声明收集。
- [di](di/README.md)：Compiler、冻结计划与 Dig 构造。
- [config](config/README.md)：标准来源、Koanf 加载与 fsnotify 监听。
- [runtime](runtime/README.md)：事务构造、生命周期、Runner 与 Reload 协调。

Runtime 只依赖显式注入 Port，不自行创建第三方引擎。具体实现的选择只能出现在
`internal/bootstrap`，从而把未来替换成本收敛到组合根。
