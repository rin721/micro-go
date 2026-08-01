# Kernel 配置 Source

本包实现 map、YAML/JSON 文件、环境变量和标准库 FlagSet 来源。

每个 Source 只读取项目 `Payload`，不合并配置。内存 map 每次 Load 深复制；`FromFile`
在创建时校验并固定绝对路径；环境变量使用双下划线映射层级，并允许组合根排除进程控制键。
这样读取事实、合并策略和监听资源拥有不同责任，可以分别替换和测试。

入口见 [`source.go`](source.go)，候选构建见 [`../koanf`](../koanf/README.md)。
