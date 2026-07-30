# Examples 入口

`examples` 保存可直接运行的应用组装示例。示例只导入项目公共包和具体 Adapter，不导入 Dig、Koanf、validator、fsnotify、Zap 或 UUID 的第三方类型。

当前示例：

- [basic](basic/README.md)：Slog、系统时钟、UUID、配置和 Runner 的最小完整组合。

示例用于证明公共使用方式，不承担内部原理说明。框架实现从 [`kernel`](../kernel/README.md) 阅读，第三方隔离从 [`adapter`](../adapter/README.md) 和 [`internal`](../internal/README.md) 阅读。

