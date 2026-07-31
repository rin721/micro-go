# 运行脚手架

在仓库根目录执行：

```powershell
go run ./cmd/app
```

`cmd/app` 把操作系统信号转换为根 Context，随后只调用 `internal/bootstrap.Run`。默认组合根
会装配 Slog、System Clock、UUID、Koanf、Dig 和 fsnotify，并加载以下优先级：

1. 代码默认值；
2. `config/app.yaml`；
3. `APP_` 环境变量。

后置来源覆盖前置来源。需要使用其他文件时设置 `APP_CONFIG_FILE`。修改应用组件、Adapter
选择或 Module 列表时进入 [`internal/bootstrap`](../../internal/bootstrap/README.md)，不要在
业务组件中直接创建基础设施实现。
