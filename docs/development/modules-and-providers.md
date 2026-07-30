# 模块与 Provider

模块只声明配置、Provider、Binding 和 Export。Provider 必须是普通构造函数，只返回一个具体组件，允许第二个结果为 error。

跨模块依赖必须经过导出接口：实现模块执行 `Bind[Contract, *Implementation]` 与 `Export[Contract]`，消费者构造函数只接收 Contract。具体类型默认私有，Registry 在注册后冻结，组件不得保存 Registry 或 Application。

需要 I/O 或 Context 的工作放在 Prepare；长期阻塞工作放在 Runner；Stop 停止活动，Close 释放最终资源。

