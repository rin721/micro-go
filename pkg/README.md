# 具体包设计

`pkg` 保存业务可以通过稳定 Capability 使用的具体实现，当前从 [`adapter`](adapter/README.md)进入。

目录名不表示永久外部 SDK 承诺；Capability Adapter 必须保持清洁导出面。Kernel 默认实现
属于应用内部机制，位于 [`internal/adapter`](../internal/adapter/README.md)。

本项目不建立无语义 `pkg/utils`。工具只有在无状态、无资源、跨两个以上包复用且没有更明确
所有者时，才允许放入 `pkg/utils/<能力名>`。

新增能力的完整流程见[Capability 封装与注入](../docs/development/capability-adapters.md)。
