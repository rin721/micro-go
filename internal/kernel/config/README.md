# internal/kernel/config

本包定义配置来源、监听描述、不可变 Snapshot 和项目自有校验契约。

## 为什么这样设计

`Source` 只读取配置事实，合并和解码留给内部 Adapter，因此调用方不依赖 Koanf。Snapshot 使用规范化 JSON 保存强类型值，每次 `Value[T]` 都重新解码，从所有权上阻止调用方修改当前配置。

## 核心入口

- `Source` 与 `WatchSource` 只定义读取和监听描述。
- 后声明的 Source 覆盖前者，类型冲突由严格合并拒绝。
- 环境变量使用 `PREFIX_`，双下划线映射层级；Flag 名称直接使用点分路径。
- `Validator` 表达不依赖第三方标签的领域校验。
- `Snapshot.Hash` 供 Reload 判断某个配置类型是否变化。

## 边界

本包不原地修改配置，不持有 Koanf 或 fsnotify 对象。标准来源、加载与监听从 [`pkg/adapter/kernel/config`](../../../pkg/adapter/kernel/config/README.md)进入。

## 验证

[`snapshot_test.go`](snapshot_test.go)验证深复制；多来源覆盖由 Koanf Adapter 测试覆盖。
