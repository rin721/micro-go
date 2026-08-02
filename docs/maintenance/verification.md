# 验证与故障定位

验证必须区分依赖解析、编译、行为、并发和实际运行，不把未执行项描述为已经通过。

## 快速文档门禁

```powershell
go test ./internal/architecture -run '^TestDocumentation'
```

该命令检查文档入口、可达性、本地链接、README 覆盖、标题、行数、目录命名和旧路径残留。

## 中文注释覆盖门禁

```powershell
go test ./internal/architecture -run '^(TestRepositoryOwnedGoCodeHasChineseComments|TestCommentCoverageRules)$'
```

门禁通过 Go AST 扫描 `cmd`、`internal`、`pkg`、`types` 中的生产、测试和示例源码，要求每个
文件具有中文职责说明，并要求所有顶层定义、函数、方法、结构体字段、嵌入字段和接口方法具有
相邻中文解释。函数内短变量和语句没有可靠的 GoDoc 归属，继续通过人工 Diff 审阅确保每个非
平凡逻辑块都可追踪；禁止用注释行比例替代语义审阅。

门禁的源码范围、文档限制和受控隔离区集中在
[`types/testing/gateconfig`](../../types/testing/gateconfig/README.md)。当前 Password 手工代码位于
`_quarantine/password`，既不参与仓库质量扫描，也因下划线目录规则不进入根 Module 的 Tidy、
Build、Test、Race 和 Vet。恢复时必须完成单轨适配并删除隔离配置，不能扩大或永久保留例外。

## 完整门禁

```powershell
./scripts/verify.ps1
```

Shell 环境使用 `./scripts/verify.sh`。完整入口调用当前地基的 unit/static 门禁：

- `verify-unit.ps1` / `verify-unit.sh`：`go mod tidy -diff`、build、禁用缓存的普通测试、禁用缓存的
  race、vet 和 `git diff --check`；普通测试中的架构门禁按全局配置执行只读源码格式检查。

所有脚本只检查，不自动修改 `go.mod`、`go.sum` 或源码。fresh test 避免先前缓存结果掩盖
当前机器上的文件事件和时序问题。CI 在 Windows/Linux 分别执行同一门禁，并额外验证项目
初始化；Linux 还验证默认应用的容器启动和 SIGTERM 退出码。

## 故障矩阵

- Collector/Compiler：nil、重名、冻结、panic、非法 Provider、可见性和循环。
- Constructor：Provider error/panic、取消、逆序 Close 和多错误保留。
- Lifecycle/Observer：每个阶段的 error、panic、协作式超时、补偿和最终状态。
- Reload：无效候选、成功提升、RestartRequired、部分应用失败、超时和文件事件。
- Config：来源优先级、控制变量排除、格式、严格合并、校验、深复制和取消。

测试通过只证明 Kernel、默认组合根和覆盖平台的行为。默认 `process`没有 HTTP、数据库、缓存
或消息协议；真实后端消费者必须在仓库外按目标项目的技术栈补充协议与故障验证。

## 修改后检查

1. 审阅完整 Diff，确认没有覆盖用户原有修改。
2. 搜索旧符号、旧路径和旧配置键，确认单轨迁移完成。
3. 先运行相关包测试，再运行完整门禁。
4. 如实报告通过项、未执行项和剩余限制。

## 制品与容器

CI 的 Linux 门禁使用根 [`Dockerfile`](../../Dockerfile)构建 `CGO_ENABLED=0`的 `cmd/app`，以
非 root、无 shell 的 scratch 镜像启动，观察真实启动日志，再由 `docker stop`发送 SIGTERM 并
断言退出码为 0。本地没有 Docker 时只能报告该门禁未执行，不能用 `go build`替代容器结论。
