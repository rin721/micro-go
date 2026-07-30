# 第三方适配器边界

`kernel` 与 `capability` 不直接导入第三方模块。Dig、Koanf、validator 和 fsnotify 位于 `internal`；Zap 与 Google UUID 位于具体 Adapter。

Dig 不决定图语义，Koanf 不拥有 Snapshot，Zap 不定义公共 Field，UUID 不作为公共值类型。内部库错误会先转换成项目错误模型。`internal/architecture` 的自动测试会阻止公共层新增第三方 import，并检查 Adapter 导出函数签名。

