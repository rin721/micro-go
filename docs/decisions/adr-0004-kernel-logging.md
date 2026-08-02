# ADR-0004：日志是唯一双阶段能力

状态：Accepted

## 背景

注册、配置加载和依赖构造发生在业务 Logger 可用之前，但这些阶段同样必须保留诊断。若 Kernel
依赖业务日志 Module 才能输出错误，日志 Provider 自身失败时会失去出口；若 Kernel 和业务各自
创建默认 Logger，又会产生配置、文件资源和关闭所有权不一致的双轨实现。

## 决策

Kernel 必须由 Bootstrap 注入一个 `logging.Manager`。默认 Manager 使用标准库 Slog，创建不依赖
应用配置的早期基线；默认日志 Module 在构造时配置并把同一个实例绑定、导出为公共
`logging.Logger`。Kernel Event 先写必有日志，再通知可选 Observer；错误字段在交给任何 Logger
前完成敏感赋值脱敏。

日志是唯一可以同时进入 Kernel 和业务图的 Capability。Clock、ID Generator 等能力继续只按
普通 Module 注入。选择 Zap 等增强实现时，Module 仍按 `Provide → Bind → Export` 声明，并显式
添加 `WithKernelLoggerReplacement[Implementation]`。Runtime 验证具体 Provider 与
`logging.Logger` Binding，且仅在全部构造成功后切换；构造失败保留基线。

Shutdown 开始前恢复 Kernel 基线，随后 Stop、Close 和最终状态只使用基线。替换实例仍由提供它
的 Module 和 Runtime 生命周期关闭；Manager 不关闭、修改或静默回退外部 Logger。Bootstrap
拥有 Kernel 基线，并在 `run` 返回时聚合其 Close 错误。

## 后果

- 注册、配置、构造、运行和关闭阶段始终有日志出口，Observer 只承担额外测试、指标或诊断。
- 默认 Kernel 与业务共享一套 Slog 配置和输出资源，不保留公共 Slog Adapter。
- 替换必须显式、单实例且通过静态图校验；`Built` 到 Shutdown 开始前的 Kernel 事件使用增强实现。
- Noop 只能静默业务 Logger；没有替换 Option 时不能关闭 Kernel 基线日志。
- Kernel Slog 可以依赖项目自有 Logging Capability，但第三方日志类型仍不得穿透项目契约。
