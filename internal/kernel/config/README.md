# internal/kernel/config

## 职责

定义配置 Source、监听描述、不可变 Snapshot、强类型读取和项目校验错误。

## 边界与失败语义

本包不持有 Koanf/fsnotify，也不原地修改配置。`Value[T]`返回深复制；`ValidationError`提供稳定
问题摘要并保留标签或领域校验原因链。缺失类型和解码失败明确返回 error。

## 关键入口

- [`Source`](source.go)、[`WatchSource`](source.go)
- [`Snapshot`](snapshot.go)、[`Value`](snapshot.go)
- [`Validator`](snapshot.go)、[`ValidationError`](snapshot.go)

## 验证

[`snapshot_test.go`](snapshot_test.go)验证深复制；完整来源与候选矩阵位于
[`internal/adapter/kernel/config`](../../adapter/kernel/config/README.md)。开发步骤见
[配置开发](../../../docs/development/configuration.md)。
