# internal/adapter/kernel/config/koanf

本包把项目 Source 适配到 Koanf，完成严格合并、强类型解码、校验、深复制和摘要生成。

## 流水线

1. 每次 Load 创建全新 Koanf 实例并启用 `StrictMerge`。
2. 按 Source 声明顺序加载 Map、JSON 或 YAML，后者覆盖前者。
3. 按稳定类型顺序解码已编译的配置声明。
4. 执行 validator 标签和项目 `Validate()` 领域校验。
5. 把 validator 错误归一化为 `ValidationIssue`。
6. 使用规范化 JSON 创建 Snapshot 深复制和 SHA-256 摘要。

## 为什么不复用 Koanf 实例

Reload 必须先生成完整候选，失败时保留旧版本。新实例既提供事务边界，也避免 Koanf 并发 Load/Get 需要额外同步。Koanf、Parser 和 validator 类型不会离开本包。

[`loader_test.go`](loader_test.go)覆盖四种来源和覆盖顺序。
