# 任一门禁失败都立即停止，避免后续成功输出掩盖最早的真实错误.
$ErrorActionPreference = "Stop"

# 快速门禁只检查格式，不在验证阶段改写工作区.
$unformatted = gofmt -l cmd internal types pkg
if ($unformatted) {
    throw "Go files require gofmt:`n$unformatted"
}

go mod tidy -diff
go build ./...
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
git diff --check
