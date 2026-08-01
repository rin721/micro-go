# internal/adapter/kernel/di/compiled

## 职责

保存 Compiler 已校验的 Provider、Binding、Config、Instance 和冻结执行 Plan。

## 边界与失败语义

结构包含反射类型、构造函数和实例，只供 Kernel Adapter 内部协作；不得作为业务 API 或运行期
Resolve 入口。无独立失败策略，非法声明必须在 Compiler 阶段被拒绝。

## 关键入口

- [`types.go`](types.go)：全部计划结构和值复制规则。
- [`Plan.Graph`](types.go)：转换后的项目只读依赖图。

## 验证

计划稳定性由 [`compiler_test.go`](../compiler/compiler_test.go)验证；构造消费方见
[`dig`](../dig/README.md)。
