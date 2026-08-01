# internal/architecture

本包使用 Go AST 自动检查目录职责，防止重构后依赖方向再次漂移。

- `types/**` 只允许标准库和同层契约。
- `internal/kernel/**` 禁止第三方库以及任何 Adapter 实现。
- Capability Adapter 禁止导入 Kernel。
- 所有 Adapter 导出契约禁止第三方类型。
- `cmd/app` 只能导入标准库和 `internal/bootstrap`。

这些测试还检查本地 Markdown 链接、源码路径以及每个 Go 包的 `README.md`，
但不替代 Compiler 的模块可见性、循环和唯一 Binding 测试。运行
`go test ./internal/architecture` 可以单独执行门禁。
