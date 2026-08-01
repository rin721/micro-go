# 仓库内部契约

脚手架不是供其他 Go Module 导入的框架 SDK。跨业务包稳定使用的契约只有：

- `types/capability/logging.Logger`
- `types/capability/clock.Clock`
- `types/capability/idgen.Generator`

`internal/kernel` 中的 Module、Config、Lifecycle、Reload、Application 状态和 Graph 属于当前
仓库内部协作协议。默认 `internal/adapter/kernel/runtime.Runtime` 通过 Bootstrap 注入 Collector、
Compiler、Loader、Constructor 和 Watcher，再执行 Compile、Build 与 Run。

错误使用 `diagnostic.ComponentError` 标记 Module、Component、Provider 和 Phase；业务 Cause
保持错误链，Panic 转换为携带堆栈的 `diagnostic.PanicError`。
