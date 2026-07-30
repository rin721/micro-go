# 创建第一个应用

应用组装根显式选择模块与配置源：

```go
application, err := app.Build(ctx,
    app.WithModules(slogadapter.Module{}, system.Module{}, myModule{}),
    app.WithConfigSources(
        config.FromValues(defaults),
        config.FromFile("configs/app.yaml"),
        config.FromEnvironment("APP"),
    ),
)
if err != nil { return err }
return application.Run(ctx)
```

后置配置源覆盖前置来源。环境变量用双下划线表示层级，例如 `APP_LOGGING__LEVEL=debug`。操作系统信号由最外层入口转换为 Context，Kernel 不直接处理信号或调用 `os.Exit`。

