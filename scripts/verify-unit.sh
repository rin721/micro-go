#!/usr/bin/env sh
# 任一命令失败或变量未定义时立即退出，保证脚本不会报告虚假的整体成功。
set -eu

# diff 模式验证依赖归类和校验和，无需修改 go.mod/go.sum。
go mod tidy -diff
# 编译所有包，覆盖没有测试用例的命令和契约代码。
go build ./...
# 禁用缓存执行普通测试和架构格式门禁，再用 race 检测并发访问。
go test -count=1 ./...
go test -race -count=1 ./...
# vet 执行 Go 静态语义检查。
go vet ./...
# 检查当前补丁中的空白错误和冲突标记。
git diff --check
