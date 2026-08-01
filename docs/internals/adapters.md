# 第三方适配器边界

- `types/**` 只依赖标准库，不接触具体实现。
- `internal/kernel/**` 不导入第三方库，也不反向导入任何 Adapter。
- 普通 Capability Adapter 只实现 `types/capability`，不依赖 Kernel 生命周期。
- `internal/adapter/kernel/**` 可以使用 Dig、Koanf、validator 和 fsnotify 实现内部协议。
- `internal/bootstrap` 使用私有桥接组件把日志关闭和配置调整接入 Kernel。

Dig 不决定图规则，Koanf 不拥有当前 Snapshot，Zap/Slog 不定义公共 Field，UUID 不进入业务
值类型。架构测试会检查 import 方向和 Adapter 导出签名，阻止第三方类型穿透边界。
