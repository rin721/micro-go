# internal/adapter/kernel/config/koanf

## 职责

从空 Koanf 树加载 Source，执行严格合并、强类型解码、标签/领域校验，并生成不可变 Snapshot。

## 边界与失败语义

每次 Load 使用新实例，失败候选不会污染当前 Snapshot。第三方类型不进入 Kernel 契约；合并、
解码和校验错误补充来源或配置上下文并保留原因链。

## 关键入口

- [`New`](loader.go)：创建 Loader。
- [`Loader.Load`](loader.go)：构建一个完整候选。

## 验证

[`loader_test.go`](loader_test.go)覆盖 Values、JSON/YAML、Environment、Flags、StrictMerge、校验、
深复制和取消；配置开发流程见[配置开发](../../../../../docs/development/configuration.md)。
