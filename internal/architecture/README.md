# internal/architecture

## 职责

使用 Go AST、`go/format`和文件系统检查依赖方向、第三方类型穿透、源码格式、单轨目录和文档结构。

## 边界与失败语义

门禁只验证结构事实，不代替 Compiler、Runtime 或业务行为测试。任何解析、链接或规则错误都
使测试失败，不自动改写源码与文档。

## 关键入口

- [`policy_test.go`](policy_test.go)：加载全局策略并统一处理跨平台路径与隔离边界。
- [`boundary_test.go`](boundary_test.go)：代码依赖与导出面门禁。
- [`format_test.go`](format_test.go)：按全局源码范围执行只读格式门禁。
- `documentation_test.go`：文档入口、可达性、格式和旧路径门禁。

## 验证

运行 `go test ./internal/architecture`执行全部门禁；仅检查文档使用
`go test ./internal/architecture -run '^TestDocumentation'`。规则说明见
[验证与故障定位](../../docs/maintenance/verification.md)。
