# 配置开发

配置流水线把多份外部事实合并成强类型、不可变的 Snapshot，再通过普通 Provider 参数注入
组件。业务组件不读取环境变量、文件或 Koanf 全局对象。

## 声明配置所有权

配置 struct 放在拥有该语义的模块附近，并使用 `yaml`/`json` 标签表达字段名。Bootstrap 中的
[`loggingModule`](../../internal/bootstrap/module_logging.go)通过
`module.Config[loggingConfig](registry, "logging")`声明所有权；Compiler
禁止其他模块直接依赖该配置类型。

字段约束可以使用 `validate` 标签；跨字段或领域规则实现项目 `config.Validator`。校验错误会
形成稳定 `ValidationIssue`，同时保留原始错误链。Loader 会拒绝配置 struct 中不存在的字段和
没有任何 Module 声明所有权的路径，避免拼写错误静默回退默认值；明确声明的 map 字段仍可
接收动态键。

## 来源与优先级

当前 Bootstrap 按以下顺序传入 Source，后者覆盖前者：

1. `FromValues`：代码默认值；
2. `FromFile`：YAML 或 JSON 文件；
3. `FromEnvironment("APP", "APP_CONFIG_FILE")`：环境变量。

`APP_CONFIG_FILE` 是进程控制变量，已从业务配置树排除。Source 只读取事实；严格合并、解码、
校验和 Snapshot 创建由 [`koanf`](../../internal/adapter/kernel/config/koanf/README.md)完成。

## 注入与读取

初次构造时，Provider 直接接收强类型配置值。运行期 Reloader 接收完整候选 Snapshot，并使用
`config.Value[T]`获取深复制；组件不能持有或修改 Snapshot 内部状态。

## 变更原则

- 新字段必须有明确所有者、单位、零值和缺失语义。
- 默认值集中在组合根或部署配置，不散落在业务读取失败分支。
- 不记录配置值、Token、DSN 或其他凭据。
- 修改配置结构时同步更新默认文件、Bootstrap、测试和本页。

Reload 的提交边界见[生命周期与 Reload](lifecycle-and-reload.md)，精确类型见
[`internal/kernel/config`](../../internal/kernel/config/README.md)。
