# 验证与故障定位

验证必须区分依赖解析、编译、行为、并发和实际运行，不把未执行项描述为已经通过。

## 快速文档门禁

```powershell
go test ./internal/architecture -run '^TestDocumentation'
```

该命令检查文档入口、可达性、本地链接、README 覆盖、标题、行数、目录命名和旧路径残留。

## 完整门禁

```powershell
./scripts/verify.ps1
```

Shell 环境使用 `./scripts/verify.sh`。完整入口依次调用两个可独立报告的门禁：

- `verify-unit.ps1` / `verify-unit.sh`：gofmt、`go mod tidy -diff`、build、禁用缓存的普通测试、
  禁用缓存的 race、vet 和 `git diff --check`。
- `verify-integration.ps1` / `verify-integration.sh`：使用 `integration`构建标签装配真实 HTTP
  Listener 与 SQLite 文件，并分别运行普通和 race 测试；Unix 额外对子进程发送 SIGTERM。

所有脚本只检查，不自动修改 `go.mod`、`go.sum` 或源码。fresh test 避免先前缓存结果掩盖
当前机器上的文件事件和时序问题。CI 将两个门禁拆成独立步骤，不能用快速门禁成功代替协议
集成结论。

## 故障矩阵

- Collector/Compiler：nil、重名、冻结、panic、非法 Provider、可见性和循环。
- Constructor：Provider error/panic、取消、逆序 Close 和多错误保留。
- Lifecycle/Observer：每个阶段的 error、panic、协作式超时、补偿和最终状态。
- Reload：无效候选、成功提升、RestartRequired、部分应用失败、超时和文件事件。
- Config：来源优先级、控制变量排除、格式、严格合并、校验、深复制和取消。
- Backend acceptance：真实 Listener、HTTP 错误边界、SQLite 迁移/事务、重启持久化、健康检查、
  SIGTERM 和 goroutine 退出。

测试通过只证明覆盖的平台和本地协议行为。默认 `process`仍没有 HTTP、数据库或消息协议；
Work Item 只证明验收 Module 的 HTTP/SQLite 纵切片，不能据此推导 PostgreSQL、Redis 或消息
协议已经可用。

## 修改后检查

1. 审阅完整 Diff，确认没有覆盖用户原有修改。
2. 搜索旧符号、旧路径和旧配置键，确认单轨迁移完成。
3. 先运行相关包测试，再运行完整门禁。
4. 如实报告通过项、未执行项和剩余限制。

## 制品与容器

CI 的 Linux 门禁使用根 [`Dockerfile`](../../Dockerfile)构建 `CGO_ENABLED=0`的 `cmd/app`，以
非 root、无 shell 的 scratch 镜像启动，观察真实启动日志，再由 `docker stop`发送 SIGTERM 并
断言退出码为 0。本地没有 Docker 时只能报告该门禁未执行，不能用 `go build`替代容器结论。
