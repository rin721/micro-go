# UUID Generator 使用说明

## 适用场景

UUID Generator 实现 [`idgen.Generator`](../../../../types/capability/idgen/idgen.go)，适合生成无需
中心协调的不透明字符串 ID。它封装 `google/uuid.NewString`，第三方 UUID 类型不会进入业务模型。

本实现不适合需要时间排序、短 ID、数据库局部性、节点编码或可预测序列的场景。

## 接入方式

组合根注册 `uuid.New`，将 `*uuid.Generator` 绑定并导出为 `idgen.Generator`。当前接入见
[`idModule`](../../../../internal/bootstrap/module_idgen.go)。消费者只依赖 Capability：

```go
func newService(ids idgen.Generator) *service {
    return &service{ids: ids}
}
```

调用 `ids.New()` 得到字符串后，业务只应把它视为不透明标识，不应解析其中版本或位布局。

## 配置与行为

本实现没有配置项、隐藏默认值或环境变量。每次 `New` 调用都委托 Google UUID 库生成标准字符串
形式的随机 Version 4 UUID。Capability 只承诺返回 `string`，业务不得依赖 UUID 版本、全局排序或
时间语义。

## 错误、并发与资源

构造和生成接口均不返回错误，这与底层 `google/uuid.NewString` 的当前 API 一致；若系统随机源
失败，底层函数会 panic，而不是返回可恢复错误。本实现无实例状态、goroutine 和需关闭资源，
可以并发共享。

测试需要稳定 ID 时，应注入实现 `idgen.Generator` 的序列或固定值替身，不要断言真实生成值。

## 示例与验证

[`example_test.go`](example_test.go) 只从公共契约验证返回非空标准长度字符串，不依赖具体值：

```text
go test ./pkg/adapter/idgen/uuid -run '^Example$'
```

第三方类型隔离和编译期接口断言见 [`uuid.go`](uuid.go)。
