# pkg/adapter/idgen/uuid

本包在内部调用 `google/uuid.NewString`，向外实现项目 `idgen.Generator`。

## 为什么二次封装

第三方 UUID 值类型不进入业务模型，消费者只接收字符串。这样更换 UUID 库或生成算法时，模块契约和配置都无需迁移。

生成器无状态、无生命周期资源，也不导入 Kernel Module。Provider、Binding 与 Export 由 Bootstrap 负责。入口见 [`uuid.go`](uuid.go)，边界由 [`internal/architecture`](../../../../internal/architecture/README.md)自动检查。
