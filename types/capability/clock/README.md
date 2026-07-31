# types/capability/clock

本包定义只包含 `Now()` 的时钟能力。

直接在业务代码调用 `time.Now` 会把时间变成不可替换的全局依赖。通过 `Clock` 参数显式注入，测试可以固定时间，生产环境则选择 [`pkg/adapter/clock/system`](../../../pkg/adapter/clock/system/README.md)。

接口刻意不包含定时器和 Sleep：当前架构只需要读取时间，不提前扩大契约。生命周期和调度仍由 Application 或组件自己管理。
