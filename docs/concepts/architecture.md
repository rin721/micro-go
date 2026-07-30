# 架构与运行链

运行链固定为：模块注册、Registry 冻结、配置加载校验、图编译、事务构造、Prepare、Start、Runner 监督、Stop、Close。

项目 Graph Compiler 拥有模块可见性、唯一绑定和稳定拓扑顺序。Dig 只是 `internal` 中的构造执行器；Koanf 只是配置合并解码器。Application 进入 Running 后，业务调用不会经过 Dig、反射、代理或字符串查找。

构造、Prepare 和 Start 失败都会进入与已完成阶段匹配的逆序清理。任何关闭错误都通过 `errors.Join` 汇总，不能阻止后续资源释放。

