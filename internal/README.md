# Internal 设计入口

`internal` 保存框架实现细节和第三方执行引擎。Go 的 internal 规则阻止外部模块导入这些包，公共扩展面只位于 `kernel` 与 `capability`。

## 配置链

1. [config/loading](config/loading/README.md)：Application 依赖的内部加载接口。
2. [config/koanfadapter](config/koanfadapter/README.md)：严格合并、解码、校验和 Snapshot 构建。
3. [config/watcher](config/watcher/README.md)：把 fsnotify 事件转换为项目 Change。

## DI 链

1. [registration](registration/README.md)：收集并冻结模块声明。
2. [di/compiler](di/compiler/README.md)：执行项目架构规则并稳定排序。
3. [di/compiled](di/compiled/README.md)：冻结的内部执行计划。
4. [di/constructor](di/constructor/README.md)：构造引擎抽象。
5. [di/digadapter](di/digadapter/README.md)：使用 Dig 事务性构造实例。

[architecture](architecture/README.md) 通过自动测试阻止第三方依赖和导出类型污染。

