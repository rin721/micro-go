# ADR-0001：第三方库只能实现项目契约

状态：Accepted

## 背景

项目需要 DI 构造、配置解析、校验、文件监听、日志和 ID 等通用能力。直接让业务 Provider
依赖 Dig、Koanf、validator、fsnotify、Zap 或 Google UUID，会把技术替换和错误处理扩散到
消费者，并形成反向依赖。

## 决策

依赖图、配置 Snapshot、生命周期、Reload 与错误语义由项目契约定义。Kernel 第三方实现
位于 `internal/adapter/kernel`，业务 Capability 实现位于 `pkg/adapter`。第三方类型不得进入
项目导出契约，具体实现只能在 Bootstrap 选择。

## 后果

- 业务 Provider 签名不因替换第三方库而改变。
- Adapter 必须统一配置、错误链、取消、资源释放和诊断。
- 项目需要维护薄封装和跨实现契约测试。
- 纯局部且不跨技术边界的逻辑不强制创建接口。
