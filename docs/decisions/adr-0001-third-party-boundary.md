# ADR-0001：第三方库只能实现项目契约

状态：Accepted。

框架选择 Dig、Koanf、validator、fsnotify、Zap 和 Google UUID 复用成熟实现，但公共 API、依赖图、配置快照、生命周期与错误语义均由项目定义。替换第三方库时只修改 `internal` 或对应 Adapter，不要求业务 Provider 改签名。

