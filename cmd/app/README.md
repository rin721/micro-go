# cmd/app

## 职责

唯一进程入口：创建信号 Context，调用 `bootstrap.Run`，并在最终失败边界输出一次错误和退出码。

## 边界与失败语义

本包不选择 Adapter、不构造业务组件，也不 Stop/Close 资源。Bootstrap 返回 error 时退出 1；
若最终错误无法写入 stderr，则退出 2。

## 关键入口

- [`main.go`](main.go)：Ctrl+C/SIGTERM、Bootstrap 调用和退出码。
- [开始运行](../../docs/getting-started/README.md)：运行与配置覆盖。

## 验证

入口 import 边界由 [`internal/architecture`](../../internal/architecture/README.md)检查；真实装配
和取消关闭由 [`bootstrap_test.go`](../../internal/bootstrap/bootstrap_test.go)验证。
