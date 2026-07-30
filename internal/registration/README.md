# internal/registration

本包执行 `Module.Register` 并把公共声明转换为带模块所有权和稳定顺序的 `Collection`。

## 为什么分成收集与编译

Registry 只记录用户意图，不同时解释 Provider 反射签名。所有类型和可见性规则统一放在 Compiler，避免注册阶段和构造阶段产生两套不一致判断。

每个模块获得独立 Registry；Register 返回后立即 Freeze。模块名必须非空且唯一，Register error 与 panic 都在构造任何资源前转为带模块上下文的错误。

输出交给 [`internal/di/compiler`](../di/compiler/README.md)，业务代码无法获得 Registry 实现。

