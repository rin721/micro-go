# 后端系统开发验证研究报告

## 结论

本轮以“复制脚手架并实现一个可持久化后端纵切片”的开发者路径重新验证仓库。当前代码已经
证明：开发者可以在同一 Module/DI/Runtime 链中声明业务契约，装配 HTTP 与 SQLite Adapter，
执行迁移和事务，暴露健康状态，并在关闭后重新构建 Application 读取原数据。

结论不是“已经具备通用生产后端”。默认 `cmd/app`仍是生命周期证明进程，Work Item 只在
`integration`测试中装配。本机 PowerShell 与 Git Bash 门禁已经通过；GitHub Actions
[run 30683789518](https://github.com/rin721/micro-go/actions/runs/30683789518)进一步证明 Windows/Linux
unit、integration 和项目初始化，以及 Linux Unix SIGTERM 和 scratch 非 root 容器启停，跨平台
交付门禁已经闭环。

## 验证范围与方法

审计日期为 2026-08-01，本机为 Go 1.25.7、Windows/amd64。验证从根 README 的开发路线开始，
按以下真实工作流推进：

1. 在独立副本替换 Module path 和应用标识，扫描旧 import。
2. 定义 Work Item 用例、消费者契约、配置和 Module。
3. 通过 Runtime 构造并启动 SQLite Store 与标准库 HTTP Server。
4. 以真实环回 Listener 请求创建、查询、幂等完成、健康检查和错误边界。
5. 关闭并重建 Application，验证 SQLite 数据仍存在。
6. 注入无效配置、Runner 提前退出、构造取消、文件监听竞争和在途请求。
7. 执行 fresh tests、race、vet、架构/文档门禁、初始化副本和交付定义检查。

当前仓库含 64 个 Go 文件、28 个测试文件和 81 个 Markdown 文件。覆盖率只作为缺口探针，
没有把单一百分比当成完成标准；关键结果包括 Runtime 80.8%、Koanf Adapter 92.1%、Bootstrap
78.3%，而验收 HTTP Adapter 为 50.4%、SQLite Adapter 为 64.6%。

## 工作流判定

| 阶段 | 判定 | 当前证据与边界 |
| --- | --- | --- |
| 项目初始化 | Windows/Linux/Git Bash 通过 | 本机双入口与 Actions matrix 均完成替换、残留扫描、build 和 Bootstrap 测试 |
| Module/DI 装配 | 通过 | 业务依赖 Repository/Readiness 契约，SQLite 与 HTTP 由验收 Module 一次性注入 |
| 配置 | 通过 | 强类型解码、Module 路径所有权、未知字段/根路径拒绝、动态 map 保留 |
| 启动与运行 | 通过 | Prepare/Start/Run 顺序、随机端口、真实 Listener、Runner 意外退出失败语义 |
| 数据一致性 | 验收级通过 | SQLite 事务迁移、创建、查询、幂等完成和 Application 重建持久化 |
| Reload | 通过但有限制 | 启动窗口重读、候选拒绝可见、旧 Snapshot 保留；跨组件部分应用仍失败退出 |
| 健康与诊断 | 验收级通过 | liveness/readiness/status、默认结构化 Runtime 事件和常见赋值脱敏 |
| 关闭 | Windows/Unix 通过 | 本机 Context 关闭和在途 HTTP drain；Actions Linux 子进程实际接收 SIGTERM 并以 0 退出 |
| 交付 | 跨平台门禁通过 | Actions Windows/Linux matrix 全绿；Linux 已证明 scratch 非 root 镜像构建、启动、SIGTERM 停止和退出码 0 |

## 本轮已经修复的缺陷

| 缺陷 | 修复结果 | 关键证据 |
| --- | --- | --- |
| Build 到 Watch 之间可能丢文件事件 | Watcher 先于 Runner 建立，并在暴露 Running 前重读候选 | `TestRunReconcilesConfigChangedBetweenBuildAndWatch`及高重复/race |
| fresh test 可失败但缓存门禁误报成功 | 单元和集成脚本统一使用 `-count=1` | `verify-unit.*`、`verify-integration.*` |
| PowerShell 原生命令失败仍可能继续执行 | 统一包装 go/git/gofmt 退出码并转为终止错误 | 退出码 23 故障探针与完整 `verify.ps1` |
| 架构门禁会扫描被忽略的临时项目副本 | 只排除仓库根 `.git`与 `tmp`，保留真实源码目录检查 | 含失败副本时 `go test -count=1 ./internal/architecture`通过 |
| 配置候选拒绝缺少默认可见出口 | Bootstrap 增加 JSON Runtime Observer，并脱敏常见凭据赋值 | Observer 与 Reload 集成测试 |
| Runner 返回 nil 被当成正常关闭 | 增加稳定 `ErrRunnerExited`并进入 Failed | 生命周期失败测试 |
| 构造 Context 取消会污染资源回滚 | Runtime 用独立 shutdown budget 逆序 Close，并聚合原始/清理错误 | 取消与 Close 故障注入测试 |
| 未知配置字段被忽略 | Module 路径所有权校验和 `ErrorUnused`严格解码 | 未知根/嵌套字段和动态 map 测试 |
| 复制项目只能手工改 `go.mod` | 增加 PowerShell/Shell 初始化、输入保护、残留扫描和新项目文档 | `tmp/init-acceptance-v2`真实副本 |
| Shell 初始化在 `/tmp`中静默跳过全部文件 | 目录排除改为只 prune 目标内部 `.git`/`tmp`，不再匹配目标绝对路径 | Git Bash 在 `tmp/shell-init.*`真实副本通过 |
| Windows checkout 的 CRLF 触发 gofmt 全树误报 | `.gitattributes`统一文本 LF，保留 gofmt 门禁 | Actions run `30683789518` Windows unit/static、integration 和初始化全绿 |
| 没有后端协议纵切片证据 | 增加 Work Item、HTTP、SQLite、迁移、事务、健康与重启持久化 | 带 `integration`标签的黑盒测试 |
| 单元和协议失败混在一个 CI 步骤 | 拆分 unit/static 与 HTTP/SQLite integration 门禁 | 双平台 workflow matrix |
| 无可复现最小运行制品定义 | 增加固定 Go 补丁版本的多阶段 scratch、非 root 镜像 | Dockerfile 与 Linux 容器门禁定义 |

## 仍然存在的缺口

### G1 默认应用还不是具体业务系统

这是脚手架定位的刻意边界，不应通过虚构 CRUD 解决。Work Item 证明基础机制可用，但新项目仍
必须选择真实业务用例，把自己的 Module 接入组合根，并删除不再需要的验收样例。完成条件由
目标项目的业务验收定义，不能由本仓库代填。

### G2 生产数据层决策尚未形成

SQLite Adapter 只验证 Repository、事务、迁移和资源所有权。它没有证明 PostgreSQL/MySQL
协议、连接池容量、迁移锁与校验、备份恢复、主从切换或数据保留策略。生产项目应按真实数据库
做 ADR，接入成熟迁移工具，并在 CI 使用真实服务执行升级、回滚/恢复与连接故障测试。

### G3 服务边界治理取决于目标项目

验收 Server 已有超时、请求体限制、严格 JSON、稳定错误映射和优雅 drain，但没有认证授权、
租户隔离、请求 ID、访问审计、速率限制、代理信任、TLS 终止策略或 API 契约生成。这些不是
无条件默认能力；一旦目标系统需要公网或多租户访问，应在首个业务接口前明确威胁边界和契约。

### G4 可观测性只达到本地诊断级

当前有业务结构化日志、Kernel Runtime 事件、liveness/readiness/status，但没有指标、Trace、
日志关联 ID、SLO 或告警。生产化计划应从目标 SLI 倒推 OpenTelemetry/指标 Adapter 与导出端，
保持业务只依赖项目 Capability，不直接穿透第三方 SDK。

### G5 Secret 与配置发布流程未闭环

配置支持文件和环境变量，Observer 对常见 `key=value`错误片段脱敏，但这不是 Secret Manager
或全量数据防泄漏证明。生产项目需要确定 Secret 来源、轮换、权限、审计和日志字段策略，并用
故障注入证明 Reload/启动失败不会输出真实凭据。

### G6 供应链和制品治理缺失

当前容器定义没有 SBOM、漏洞扫描、签名、来源证明或发布标签策略，GitHub Actions 也只固定
主版本而非不可变 commit。上线前应固定 Actions SHA，增加依赖/镜像扫描、SBOM 与 provenance，
并把发布制品与源提交建立可追溯关系。

### G7 验收强度仍有提升空间

门禁没有覆盖率阈值，HTTP/SQLite Adapter 的错误分支仍有未覆盖区域；也没有负载、长时间运行、
磁盘耗尽、只读文件系统、时钟跳变或真实外部数据库/缓存/消息故障。应按风险补测试，不以提高
数字为目的；优先覆盖生产选择的协议和失败模式。

## 分阶段修复计划

| 优先级 | 工作 | 完成条件 |
| --- | --- | --- |
| P0 首个业务切片 | 用目标项目的真实用例替换默认 `process`，决定是否保留验收样例 | 业务契约、Module、配置、失败路径、协议黑盒和文档同时落地，无双轨入口 |
| P1 上线前 | 决定认证授权、数据库/迁移、Secret、TLS/代理和 API 契约 | ADR/契约明确，真实依赖集成与故障恢复门禁通过 |
| P1 上线前 | 接入 SLI 驱动的日志、指标、Trace 与告警 | 请求可关联，关键依赖和生命周期有指标，告警演练可观察 |
| P1 发布前 | 固定 CI Action SHA，增加 SBOM、扫描、签名和 provenance | 高危问题有阻断策略，制品可追溯到提交和构建 |
| P2 容量确认 | 执行负载、资源上限、长稳、磁盘/网络故障和恢复演练 | 达到目标项目明确的吞吐、延迟、恢复时间与数据一致性指标 |

## 最终判定

作为“开发者拿来继续做后端项目”的地基，仓库已从抽象 Kernel 基线推进到有真实后端纵切片、
项目初始化和分层门禁的可用状态，最初发现的 Runtime 正确性缺陷已经闭环。作为“可直接上线的
通用后端系统”，结论仍是不成立：跨平台交付门禁已经闭环，但其余工作必须由目标业务、真实
基础设施与生产 SLO 驱动，不能把验收纵切片等同于生产能力。
