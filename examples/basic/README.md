# examples/basic

这是可以直接运行的最小 micro-go 应用：

```powershell
go run ./examples/basic
```

## 组装过程

1. 组合根显式选择 Slog、System Clock 和 UUID 三个 Adapter Module。
2. `componentModule` 登记普通 `NewComponent` Provider。
3. 内存 Source 提供 `logging` 强类型配置。
4. `app.Build` 编译并构造全部依赖。
5. `app.Run` 监督 Component.Run；示例 Runner 输出一条日志后正常返回并触发关闭。

## 为什么这样写

`Component` 只依赖 `logging.Logger`、`clock.Clock` 和 `idgen.Generator`，不知道具体第三方实现。只有 main 组合根了解 Adapter 选择，因此将 Slog 替换为 Zap 不需要改业务组件。

入口源码见 [`main.go`](main.go)。更完整的模块规则见 [`kernel/module`](../../kernel/module/README.md)。

