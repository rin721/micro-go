# 任一门禁失败都立即停止，避免后续成功输出掩盖最早的真实错误.
$ErrorActionPreference = "Stop"
# 点导入只引入统一原生命令检查函数，不启动额外 PowerShell 进程。
. "$PSScriptRoot/invoke-native.ps1"

# 快速门禁只检查格式，不在验证阶段改写工作区.
$unformatted = Invoke-NativeCommand -Command { gofmt -l cmd internal types pkg } -Description "gofmt check"
# gofmt -l 输出任意文件都代表格式漂移，验证阶段不自动改写用户工作区。
if ($unformatted) {
    throw "Go files require gofmt:`n$unformatted"
}

# tidy 的 diff 模式只检查依赖清单，不在门禁中修改 go.mod 或 go.sum。
Invoke-NativeCommand -Command { go mod tidy -diff } -Description "go mod tidy -diff"
# build 编译所有包，覆盖无测试文件的命令和契约包。
Invoke-NativeCommand -Command { go build ./... } -Description "go build"
# 禁用测试缓存，确保当前源码和文件系统事实被重新执行。
Invoke-NativeCommand -Command { go test -count=1 ./... } -Description "go test"
# race 再执行完整测试集合，验证 Runtime、Watcher 和 Logger 同步边界。
Invoke-NativeCommand -Command { go test -race -count=1 ./... } -Description "go test -race"
# vet 检查编译器不会拒绝但可能表示错误的 Go 结构。
Invoke-NativeCommand -Command { go vet ./... } -Description "go vet"
# 最后检查补丁空白和冲突标记；放在末尾仍由统一退出码检查。
Invoke-NativeCommand -Command { git diff --check } -Description "git diff --check"
