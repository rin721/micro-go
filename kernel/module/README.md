# kernel/module

本包定义模块注册的公共语言。模块只声明“可以怎样构造”和“允许谁依赖”，不直接创建或查询实例。

## 为什么这样设计

显式 `Module.Register` 比自动扫描更容易审查和复现。Registry 在注册结束后冻结，能够阻止运行期偷偷修改依赖图；模块之间只通过已绑定并导出的接口协作，具体实现保持私有。

## 核心入口

- [`Module`](module.go)：提供稳定名称并登记声明。
- `Provide`：登记普通 Go 构造函数。
- `Bind`：把当前模块的具体实现绑定到接口。
- `Export`：允许其他模块使用该接口。
- `Config`：声明当前模块拥有的强类型配置路径。

## 不负责

本包不校验反射签名、不执行构造函数，也不提供 Resolver 或 Service Locator。这些规则分别由 [`internal/di/compiler`](../../internal/di/compiler/README.md) 和构造适配器执行。

## 验证

模块可见性、重复绑定和非法 Provider 由 [`kernel/app` 测试](../app/app_test.go)覆盖。

