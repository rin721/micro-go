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

Shell 环境使用 `./scripts/verify.sh`。两个脚本只检查，不自动修改 `go.mod`、`go.sum` 或源码。
完整顺序是 gofmt、`go mod tidy -diff`、build、普通测试、race、vet 和 `git diff --check`。

## 故障矩阵

- Collector/Compiler：nil、重名、冻结、panic、非法 Provider、可见性和循环。
- Constructor：Provider error/panic、取消、逆序 Close 和多错误保留。
- Lifecycle/Observer：每个阶段的 error、panic、协作式超时、补偿和最终状态。
- Reload：无效候选、成功提升、RestartRequired、部分应用失败、超时和文件事件。
- Config：来源优先级、控制变量排除、格式、严格合并、校验、深复制和取消。

测试通过只证明覆盖的本地行为。默认 `process` 没有 HTTP、数据库或消息协议，因此不能从
当前门禁推导这些外部能力已经可用。

## 修改后检查

1. 审阅完整 Diff，确认没有覆盖用户原有修改。
2. 搜索旧符号、旧路径和旧配置键，确认单轨迁移完成。
3. 先运行相关包测试，再运行完整门禁。
4. 如实报告通过项、未执行项和剩余限制。
