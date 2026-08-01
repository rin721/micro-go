# internal/adapter/kernel/config/fsnotify

## 职责

监听配置文件父目录，并把 write、create、rename 转换为项目 `config.Change`。

## 边界与失败语义

本包不去抖、不重建候选，也不调用业务 Reloader。goroutine 绑定调用方 Context；初始化失败会
同时保留 fsnotify 错误和 Close 错误，重复事件允许非阻塞合并。

## 关键入口

- [`Watcher.Watch`](watcher.go)：实例 Port 入口。
- [`Watch`](watcher.go)：目录监听与事件过滤。

## 验证

[`watcher_test.go`](watcher_test.go)覆盖缺失目录、取消关闭和原子文件替换；上层去抖见
[Runtime 执行链](../../../../../docs/maintenance/kernel-runtime.md)。
