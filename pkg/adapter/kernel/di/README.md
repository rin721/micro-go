# Kernel DI Adapter

本目录把模块声明转换为可重复执行的依赖计划：

- [compiler](compiler/README.md)拥有项目图规则与稳定排序。
- [compiled](compiled/README.md)保存冻结反射计划。
- [dig](dig/README.md)只按计划构造实例。

Compiler 与 Dig 刻意分离，防止更换容器时连同模块可见性、唯一绑定和生命周期顺序一起变化。
