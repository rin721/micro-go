# internal/bootstrap

## 职责

唯一组合根：选择 Kernel/Capability Adapter、配置来源、Module、超时和默认业务 Runner。

## 边界与失败语义

业务组件不承担装配；Adapter 也不反向导入 Kernel Module。`Run`返回前已完成 Stop、等待 Runner
和 Close。配置或构造失败发生在业务运行前，运行错误完整返回进程入口。Bootstrap 在业务
Logger 由 Bootstrap 在读取配置前创建，并作为 Runtime 的必填 Kernel 依赖；配置拒绝、Reload、
Runner 和最终状态始终经该基线输出，诊断错误中的常见敏感赋值会先脱敏。Runtime 构造的
Kernel Logger 也通过默认日志 Module 导出给业务组件，关闭所有权仍只属于 Bootstrap。

## 关键入口

- [`Run`](bootstrap.go)：构造默认 Runtime 并驱动 Application。
- [`loggingModule`](module_logging.go)、[`clockModule`](module_clock.go)、
  [`idModule`](module_idgen.go)、[`applicationModule`](module_application.go)：当前模块集合；每个模块
  在独立文件中声明自己的配置、Provider、Binding、Export 和必要的生命周期桥接。

`applicationModule`拥有 `application.name`，初始化脚本会在新项目副本中替换其默认值和部署
配置，运行日志使用该值区分应用身份。

## 验证

[`bootstrap_test.go`](bootstrap_test.go)使用真实默认栈验证启动、取消、缺失配置和日志 Reload；
应用接入流程见[组件接入工作流](../../docs/development/component-workflow.md)。
