#!/usr/bin/env sh
# 任一命令失败或变量未定义时立即退出，保证脚本不会报告虚假的整体成功。
set -eu

# 只检查格式，不在验证阶段改写工作区。
unformatted="$(gofmt -l cmd internal types pkg)"
if [ -n "$unformatted" ]; then
  echo "Go files require gofmt:" >&2
  echo "$unformatted" >&2
  exit 1
fi

# -diff 只验证依赖文件，不允许门禁静默改写 go.mod 或 go.sum。
go mod tidy -diff
go build ./...
go test ./...
# 竞态测试单独运行，以覆盖 Runner、Observer 和配置监听的并发路径。
go test -race ./...
go vet ./...
# 最后检查补丁空白，保证本地与 CI 都能发现行尾和冲突标记问题。
git diff --check
