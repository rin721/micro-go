#!/usr/bin/env sh
# 任一命令失败或变量未定义时立即退出，保证脚本不会报告虚假的整体成功。
set -eu

# 只检查格式，不在验证阶段改写工作区。
unformatted="$(gofmt -l kernel internal capability adapter examples)"
if [ -n "$unformatted" ]; then
  echo "Go files require gofmt:" >&2
  echo "$unformatted" >&2
  exit 1
fi

# 按依赖、构建、测试、竞态和静态分析的顺序执行完整门禁。
go mod tidy
go build ./...
go test ./...
# 竞态测试单独运行，以覆盖 Runner、Observer 和配置监听的并发路径。
go test -race ./...
go vet ./...
