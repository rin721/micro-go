# Internal Adapter 设计入口

`internal/adapter` 保存应用内部协议的默认执行实现。这里可以封装第三方库，但不是外部
Module 可导入的 SDK，也不能定义业务 Capability。

- [kernel](kernel/README.md)：模块收集、图编译、配置加载、构造和 Runtime 协调。

具体技术选择只能由 [`internal/bootstrap`](../bootstrap/README.md) 完成；
[`internal/kernel`](../kernel/README.md) 只拥有协议和值模型，不反向导入本目录。
