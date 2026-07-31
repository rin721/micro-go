# 具体包设计

`pkg` 保存脚手架可以替换或独立测试的具体实现，当前从 [`adapter`](adapter/README.md)进入。

目录名不表示永久外部 SDK 承诺：普通 Capability Adapter 保持清洁导出面；
`adapter/kernel` 是当前仓库的默认 Kernel 实现，只由组合根装配。

本项目不建立无语义 `pkg/utils`。工具只有在无状态、无资源、跨两个以上包复用且没有更明确
所有者时，才允许放入 `pkg/utils/<能力名>`。
