# internal/adapter/kernel/di/compiler

## 职责

把注册 Collection 编译为稳定 Plan，并拥有项目全部静态依赖图规则。

## 边界与失败语义

本包拒绝非法/重复 Provider、配置或 Binding，缺失依赖，未导出接口，跨模块具体类型，Provider
环和模块环。错误顺序稳定；Dig 不得重新定义这些规则。

## 关键入口

- [`New`](compiler.go)：创建无状态 Compiler。
- [`Compile`](compiler.go)：校验声明、生成拓扑顺序与只读 Graph。

## 验证

[`compiler_test.go`](compiler_test.go)覆盖非法签名、两类循环和重复编译稳定性；整体模型见
[架构与运行链](../../../../../docs/concepts/architecture.md)。
