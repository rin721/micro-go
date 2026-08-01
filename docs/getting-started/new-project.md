# 从模板创建后端项目

本页把“复制后继续开发”落实为可重复、可失败的项目初始化流程。初始化会修改目标副本中的
Module path、Markdown 项目名和默认 `application.name`，并执行构建与 fresh tests；它不会修改
原模板目录，也不会创建业务占位代码。

## 1. 准备独立副本

先把仓库复制或克隆到新的项目目录，并在该副本建立自己的 Git 基线。不要把初始化命令直接
指向仍用于同步模板更新的原仓库。

目标目录必须包含 `go.mod` 和 `AGENTS.md`，并且当前 Module 必须仍是
`github.com/rin721/micro-go`，或已经等于本次传入的新 Module。脚本拒绝磁盘根目录和其他
Module，避免误改无关工程。

## 2. 初始化身份

PowerShell：

```powershell
./scripts/init-project.ps1 `
  -ModulePath example.com/team/order-service `
  -ApplicationName order-service `
  -TargetDirectory <project-copy>
```

Shell：

```sh
sh ./scripts/init-project.sh \
  example.com/team/order-service \
  order-service \
  <project-copy>
```

`ModulePath`必须包含路径分隔符；`ApplicationName`只接受小写字母、数字和连字符，且必须以
字母开头。默认会执行 `go mod tidy`、build、fresh tests 和 Bootstrap 启停测试；只有诊断脚本
本身时才使用 `-SkipVerify`或`--skip-verify`。

## 3. 检查结果

在新项目目录执行：

```powershell
rg -n "github.com/rin721/micro-go" -g "*.go" -g "go.mod"
go mod tidy -diff
go build ./...
go test -count=1 ./...
go test -race -count=1 ./...
```

第一次搜索必须无结果。默认配置中的 `application.name`、启动日志的 `application`字段和根
README 标题应使用新名称；`APP_`仍是稳定配置环境变量前缀，不随项目展示名隐式变化。

## 4. 开始真实业务切片

初始化只改变项目身份，不生成假 Controller、Repository 或数据。下一步选择一个有真实验收
条件的业务用例，按[组件接入工作流](../development/component-workflow.md)定义消费者契约、
Module、配置、生命周期和 Adapter，再用[验证流程](../maintenance/verification.md)补齐行为与
协议证据。
