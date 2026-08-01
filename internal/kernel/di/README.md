# internal/kernel/di

## 职责

提供与容器无关、可复制和可序列化的只读依赖图值模型。

## 边界与失败语义

Graph 用于诊断与审计，不提供运行期 Resolve、实例或反射构造函数。导出 Text、DOT、JSON 失败
时返回 error；调用方获得切片副本，不能修改内部 Plan。

## 关键入口

- [`Node`](graph.go)、[`Edge`](graph.go)、[`Graph`](graph.go)
- [`Graph.Text`](graph.go)、[`Graph.DOT`](graph.go)、[`Graph.JSON`](graph.go)

## 验证

顺序、边方向和稳定输出由 [`compiler_test.go`](../../adapter/kernel/di/compiler/compiler_test.go)及
[`app_test.go`](../../adapter/kernel/runtime/app_test.go)覆盖。
