# Password 受控隔离区

## 当前状态

本目录自 2026-08-02 起保存尚未适配当前脚手架的 Password 手工代码。它不是当前 Capability、
Adapter 或应用依赖图的一部分，也不参与根 Module 的格式、依赖、构建、测试、Race、Vet、
架构和文档门禁。

## 原始位置

- `pkg/adapter/password` 已迁至 `pkg/adapter/password` 子树。
- `types/capability/password` 已迁至 `types/capability/password` 子树。

外层 `_quarantine` 名称使 Go 工具忽略整棵目录；项目门禁还会按
`types/testing/gateconfig`中的准确目录边界跳过它，防止其他扫描重新把隔离代码当成当前事实。

## 隔离原因与责任

该代码来自手工复制，契约方法、Adapter 返回类型、注释、文档和 DI 装配尚未与当前项目一致。
责任主体为 repository maintainers。隔离不是兼容层，也不允许当前代码导入这里的包。

## 恢复条件

只有同时满足以下条件才允许单轨迁回原位置：

1. 由真实消费者确认 Password Capability 的最小契约。
2. Adapter 直接实现项目契约并返回可注册的具体类型，不保留重复接口。
3. 在 Bootstrap 完成配置、Provider、Binding、Export、Module 选择和消费者注入。
4. 更新当前文档、测试和依赖归类，并通过完整门禁。
5. 删除 `gateconfig`中的隔离项和本隔离目录，不保留两份实现。
