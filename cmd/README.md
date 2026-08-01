# 进程入口设计

`cmd` 只保存可执行程序入口。当前脚手架只有 [`app`](app/README.md)，禁止再建立与它并行的
示例入口，否则信号、退出码和装配方式会形成双轨。

入口不得选择具体业务依赖或关闭组件资源；这些责任统一交给 `internal/bootstrap` 和 Runtime。
运行入口见[开始运行](../docs/getting-started/README.md)，进程内部链路见
[Runtime 执行链](../docs/maintenance/kernel-runtime.md)。
