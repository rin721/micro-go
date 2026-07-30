# Kernel 设计入口

`kernel` 保存框架拥有的稳定公共契约。这里不导入 Dig、Koanf、Zap 等第三方模块，调用方可以在不绑定具体实现的情况下编写模块和组件。

## 阅读顺序

1. [module](module/README.md)：模块如何声明 Provider、Binding、Export 和配置。
2. [di](di/README.md)：编译结果如何以项目自有依赖图呈现。
3. [config](config/README.md)：配置来源和不可变 Snapshot。
4. [lifecycle](lifecycle/README.md)：组件可选实现的五个生命周期接口。
5. [app](app/README.md)：如何把注册、配置、构造、运行和关闭串起来。

辅助契约包括 [diagnostic](diagnostic/README.md)、[reload](reload/README.md) 和 [testkit](testkit/README.md)。具体构造和配置引擎位于 [`internal`](../internal/README.md)，可替换能力位于 [`capability`](../capability/README.md)。

## 核心边界

- Kernel 负责规则和状态机，不负责选择第三方实现。
- 依赖图在 Build 前冻结，运行期没有容器查询入口。
- 错误、事件、配置和图模型都是项目类型，第三方类型不得进入公共签名。

