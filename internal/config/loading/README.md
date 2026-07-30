# internal/config/loading

本包定义 Application 与具体配置引擎之间的内部 `Loader` 接口。

`Loaded` 同时携带不可变公共 Snapshot 和构造注入所需的强类型反射值。两者必须来自同一次解码，避免组件初始配置与框架报告的版本不一致。

该接口位于 internal，因为 `compiled.Config` 和 `reflect.Value` 都是实现细节。当前实现是 [`koanfadapter`](../koanfadapter/README.md)，公共调用方只接触 [`kernel/config`](../../../kernel/config/README.md)。

