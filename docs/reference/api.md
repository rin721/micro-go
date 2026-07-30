# 公共 API

- `app.Compile(options...)`：注册、加载配置并编译依赖图，不构造实例。
- `app.Build(ctx, options...)`：事务构造全部 Application Singleton。
- `Application.Run(ctx)`：只允许一次，负责生命周期与退出。
- `Plan.DependencyGraph()`：可输出 Text、DOT 和 JSON。
- `config.Value[T](snapshot)`：返回配置深复制，主要供 Reloader 使用。
- `app.WithConfigWatch()`：显式启用文件监听；未启用时没有 watcher Goroutine。

错误通过 `diagnostic.ComponentError` 标记 Module、Component、Provider 和 Phase；Panic 转换为携带堆栈的 `diagnostic.PanicError`。

