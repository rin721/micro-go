# ADR-0001：第三方库只能实现项目契约

状态：Accepted。

项目选择 Dig、Koanf、validator、fsnotify、Zap 和 Google UUID 复用成熟实现，但依赖图、配置
快照、生命周期与错误语义均由项目定义。Kernel 第三方实现位于 `internal/adapter`，业务
Capability 实现位于 `pkg/adapter`；替换技术时只修改对应 Adapter 与组合根，不要求业务
Provider 改签名。
