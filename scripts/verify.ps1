# 任一门禁失败都立即停止，避免后续成功输出掩盖最早的真实错误.
$ErrorActionPreference = "Stop"

# 先检查格式而不自动改写，让 CI 与本地都能发现未提交的 gofmt 差异.
$unformatted = gofmt -l cmd internal types pkg
if ($unformatted) {
    throw "Go files require gofmt:`n$unformatted"
}

# -diff 只验证依赖文件，不允许门禁静默改写 go.mod 或 go.sum.
go mod tidy -diff
go build ./...
go test ./...
# race 单独执行所有测试，用真实同步检测补足普通测试无法发现的数据竞争.
go test -race ./...
go vet ./...
# 最后检查补丁空白，保证本地与 CI 都能发现行尾和冲突标记问题.
git diff --check
