# types/capability/clock

## 职责

定义业务读取当前时间所需的最小 `Clock`契约。

## 边界与失败语义

接口只包含 `Now()`，不提前加入 Timer 或 Sleep。它没有资源、配置或错误语义；调度和生命
周期由组件与 Application 管理。

## 关键入口

- [`Clock`](clock.go)：返回 `time.Time`的能力接口。
- [`system`](../../../pkg/adapter/clock/system/README.md)：当前生产实现。

## 验证

System Adapter 的编译期断言保证契约同步；新实现的接入规则见
[Capability 与 Adapter](../../../docs/development/capability-adapters.md)。
