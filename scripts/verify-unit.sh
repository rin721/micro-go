#!/usr/bin/env sh
# 任一命令失败或变量未定义时立即退出，保证脚本不会报告虚假的整体成功。
set -eu

# 快速门禁只检查格式，不在验证阶段改写工作区。
unformatted="$(gofmt -l cmd internal types pkg)"
if [ -n "$unformatted" ]; then
  echo "Go files require gofmt:" >&2
  echo "$unformatted" >&2
  exit 1
fi

go mod tidy -diff
go build ./...
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
git diff --check
