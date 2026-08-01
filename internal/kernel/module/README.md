# internal/kernel/module

## 职责

定义 Module 注册语言：Provider、Binding、Export 和强类型 Config 声明。

## 边界与失败语义

Module 只声明构造与可见性，不创建或查询实例。Registry 仅在 Register 期间可写；nil Registry
立即返回 error。反射签名、唯一性、可见性和循环由 Compiler 集中校验。

## 关键入口

- [`Module`](module.go)、[`Registry`](module.go)
- [`Provide`](module.go)、[`Bind`](module.go)、[`Export`](module.go)、[`Config`](module.go)

## 验证

收集与冻结见 [`collector_test.go`](../../adapter/kernel/module/collector_test.go)，图规则见
[`compiler_test.go`](../../adapter/kernel/di/compiler/compiler_test.go)。使用流程见
[组件接入工作流](../../../docs/development/component-workflow.md)。
