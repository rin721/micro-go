# internal/architecture

本包使用 Go AST 自动检查目录职责，防止重构后依赖方向再次漂移。

- `types/**` 只允许标准库和同层契约。
- `internal/kernel/**` 禁止第三方库和 `pkg/adapter`。
- Capability Adapter 禁止导入 Kernel。
- 所有 Adapter 导出契约禁止第三方类型。
- `cmd/app` 只能导入标准库和 `internal/bootstrap`。

这些测试检查结构边界，不替代 Compiler 的模块可见性、循环和唯一 Binding 测试。运行
`go test ./internal/architecture` 可以单独执行门禁。
