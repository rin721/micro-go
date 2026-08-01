# internal/acceptance/adapter/sqliteworkitems

## 职责

使用 `database/sql`与 modernc SQLite 实现 Work Item Repository 和 Readiness。

## 边界与失败语义

Store 在 Prepare 中打开单连接池并执行版本化事务迁移，在 Close 中释放连接。SQL、驱动错误和
迁移细节不进入业务契约；事务失败同时保留主错误与 Rollback 错误。

## 关键入口

- [`Store.Prepare`](store.go)：连接与迁移。
- [`Store.Create`](store.go)、[`Store.Get`](store.go)、[`Store.Complete`](store.go)
- [`Store.Ready`](store.go)、[`Store.Close`](store.go)

## 验证

[`store_test.go`](store_test.go)覆盖迁移版本、持久化重开、NotFound 和关闭后 readiness；HTTP
黑盒集成验证位于 [`backend_acceptance_test.go`](../../../bootstrap/backend_acceptance_test.go)。
