# types/capability/idgen

本包定义返回 `string` 的 ID 生成契约。

使用字符串作为边界类型，是为了防止业务实体和 Provider 签名依赖某个 UUID 库。当前实现是 [`pkg/adapter/idgen/uuid`](../../../pkg/adapter/idgen/uuid/README.md)，以后替换算法时不需要修改消费者。

本包不承诺 ID 排序、时间编码或分布式节点语义；当前契约只保证每次调用生成一个实现定义的字符串 ID。
