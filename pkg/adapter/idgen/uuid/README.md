# pkg/adapter/idgen/uuid

## 职责

封装 `google/uuid.NewString`并实现项目 `idgen.Generator`。

## 边界与失败语义

第三方 UUID 类型不离开本包，消费者只接收字符串。本实现无状态、无生命周期资源，不承诺
排序、时间编码或节点语义；Provider、Binding 和 Export 由 Bootstrap 声明。

## 关键入口

- [`New`](uuid.go)：创建 Generator。
- [`Generator.New`](uuid.go)：生成实现定义的字符串 ID。

## 验证

编译期接口断言保证实现匹配 [`idgen.Generator`](../../../../types/capability/idgen/README.md)；导出面
由 [`internal/architecture`](../../../../internal/architecture/README.md)检查。
