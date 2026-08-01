# 术语表

| 术语 | 在本项目中的含义 |
| --- | --- |
| Application | 一次性运行的单进程状态机和资源所有者 |
| Adapter | 把标准库或第三方实现收敛到项目契约的边界实现 |
| Binding | 同一具体实例到接口契约的类型别名，不创建第二个实例 |
| Bootstrap | 唯一组合根，选择 Module、Adapter、Source 和超时 |
| Capability | 由业务消费者需求定义、实现无关的小接口 |
| Collection | Collector 生成、带模块所有权和稳定声明顺序的注册结果 |
| Component | Provider 构造并由 Application 管理的对象 |
| Export | 允许其他模块依赖当前模块已绑定接口的声明 |
| Kernel | 应用内部的配置、DI、生命周期、Reload 和诊断协议 |
| Module | 启动期声明 Provider、Binding、Export 和 Config 的注册单元 |
| Plan | Compiler 生成的冻结执行计划 |
| Provider | 返回一个具体组件、可附带 error 的普通 Go 构造函数 |
| Registry | Module.Register 期间可写、随后冻结的声明接口 |
| Reload | 从完整候选配置到组件决定和最终状态的协调过程 |
| Runner | 全部 Start 成功后由 Application 并发监督的长期任务 |
| Snapshot | 一次完整加载和校验后发布的不可变配置事实 |

同一术语不得在其他文档中被赋予第二种含义。精确接口见[契约速查](contracts.md)。
