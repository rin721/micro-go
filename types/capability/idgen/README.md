# types/capability/idgen

## 职责

定义业务生成字符串 ID 所需的最小 `Generator`契约。

## 边界与失败语义

接口返回 `string`，不泄露第三方 UUID 类型，也不承诺排序、时间编码或分布式节点语义。当前
契约没有配置、资源和 error 返回值。

## 关键入口

- [`Generator`](idgen.go)：生成字符串 ID。
- [`uuid`](../../../pkg/adapter/idgen/uuid/README.md)：当前实现。

## 验证

UUID Adapter 的编译期断言保证契约同步；扩展能力前先遵守
[新能力准入条件](../../../docs/roadmap/evolution.md)。
