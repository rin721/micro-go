# types/testing/gateconfig

## 职责

集中保存仓库质量门禁可调整的强类型策略，包括源码范围、文档限制和受控隔离区。

## 边界与失败语义

配置只使用标准库类型，不提供环境变量覆盖，也不能关闭依赖方向等固定架构不变量。
`Current`返回深复制快照，非法路径或缺失隔离治理信息由`Validate`拒绝。

## 关键入口

- `Policy`：一次门禁执行的完整策略。
- `Current`：取得不共享可变切片的当前快照。
- `Validate`：验证路径、限制和隔离元数据。
- `Policy.Excludes`：按目录边界判断忽略路径。

## 验证

运行 `go test ./types/testing/gateconfig`，再运行 `go test ./internal/architecture`验证全部消费者。
