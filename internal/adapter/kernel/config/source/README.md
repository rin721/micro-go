# internal/adapter/kernel/config/source

## 职责

提供 Map、YAML/JSON 文件、环境变量和标准库 FlagSet 四类项目 Source。

## 边界与失败语义

Source 只读取配置事实，不合并或解码。文件路径在构造 Source 时规范化；Map 每次 Load 深复制；
所有 Source 尊重 Context；环境 Source 可排除进程控制键。

## 关键入口

- [`FromValues`](source.go)、[`FromFile`](source.go)
- [`FromEnvironment`](source.go)、[`FromFlags`](source.go)

## 验证

[`source_test.go`](source_test.go)覆盖文件路径、JSON、控制变量排除、取消和深复制；来源顺序见
[配置开发](../../../../../docs/development/configuration.md)。
