# 任一门禁失败都立即停止，避免后续成功输出掩盖最早的真实错误.
$ErrorActionPreference = "Stop"

# 先检查格式而不自动改写，让 CI 与本地都能发现未提交的 gofmt 差异.
$unformatted = gofmt -l kernel internal capability adapter examples
if ($unformatted) {
    throw "Go files require gofmt:`n$unformatted"
}

# tidy 同时验证依赖图可解析；随后从编译、功能、并发到静态分析逐步收紧门禁.
go mod tidy
go build ./...
go test ./...
# race 单独执行所有测试，用真实同步检测补足普通测试无法发现的数据竞争.
go test -race ./...
go vet ./...
