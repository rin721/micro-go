# 开始运行

本页帮助第一次进入仓库的开发者启动当前脚手架，并判断运行结果是否符合真实实现。

## 前置条件

- 使用 [`go.mod`](../../go.mod)声明的 Go 1.25 或兼容版本。
- 在仓库根目录运行命令。
- 默认配置文件 [`config/app.yaml`](../../config/app.yaml)可读取。

## 启动与停止

```powershell
go run ./cmd/app
```

默认 `application.process` 会记录一条包含实例 ID 和当前时间的启动日志，然后等待根 Context
取消。它不会监听 HTTP 端口，也不会连接数据库或消息系统。按 Ctrl+C 后，进程入口取消根
Context，Runtime 等待 Runner 退出并释放组件，成功时不返回错误。

## 覆盖配置

配置优先级从低到高为：代码默认值、配置文件、`APP_` 环境变量。双下划线表示层级：

```powershell
$env:APP_LOGGING__LEVEL = "debug"
go run ./cmd/app
```

`APP_CONFIG_FILE` 只选择配置文件，不会进入业务配置树：

```powershell
$env:APP_CONFIG_FILE = "D:\config\app.yaml"
go run ./cmd/app
```

当前日志配置字段以 [`config/app.yaml`](../../config/app.yaml)和
[`loggingConfig`](../../internal/bootstrap/bootstrap.go)为准。

## 常见失败

| 现象 | 直接原因 | 检查位置 |
| --- | --- | --- |
| `read config file` | 文件不存在或不可读 | `APP_CONFIG_FILE` 与当前目录 |
| 配置校验失败 | level、output 等字段不满足约束 | 配置文件和环境变量覆盖 |
| 启动后立即退出 | Runner 返回、组件失败或配置要求重启 | 终端最终错误与 Runtime 事件 |
| 修改配置后退出 | 变化不能安全原地应用 | [Reload 语义](../development/lifecycle-and-reload.md) |

运行事实由 [`bootstrap_test.go`](../../internal/bootstrap/bootstrap_test.go)验证。下一步从
[应用开发路线](../development/README.md)理解现有组件如何进入运行链。
