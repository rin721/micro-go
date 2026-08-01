# internal/adapter/kernel/di/dig

## 职责

使用临时 Dig 容器按已编译 Plan 事务性构造组件实例。

## 边界与失败语义

Dig 只执行计划，不决定可见性、循环或顺序。Provider error/panic 会转换为项目错误，并把此前
构造的实例连同错误交还 Runtime；Runtime 使用独立 shutdown budget 逆序 Close，构造根因和
清理错误通过 `errors.Join`同时保留。成功后不保留容器。

## 关键入口

- [`New`](constructor.go)：创建构造引擎。
- [`Engine.Construct`](constructor.go)：注册配置/Provider/别名并逐个取出实例。

## 验证

[`constructor_test.go`](constructor_test.go)覆盖取消时返回已构造实例；Provider panic、独立
回滚 Context 和多错误由
[`app_test.go`](../../runtime/app_test.go)覆盖。
