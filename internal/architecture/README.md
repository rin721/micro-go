# internal/architecture

本包保存可执行的架构边界测试，而不是运行时代码。

## 门禁

- `kernel/**` 与 `capability/**` 不得导入第三方模块。
- `adapter/**` 可以内部使用第三方库，但导出类型和函数签名不得出现第三方包类型。

## 为什么自动检查

“不要污染公共 API”如果只写在文档中，很容易在新增字段或辅助函数时被破坏。测试读取源码 import 并检查导出类型，把二次封装边界变成每次 `go test ./...` 都会执行的硬约束。

测试入口见 [`boundary_test.go`](boundary_test.go)。

