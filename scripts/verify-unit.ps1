# 任一门禁失败都立即停止，避免后续成功输出掩盖最早的真实错误.
$ErrorActionPreference = "Stop"
. "$PSScriptRoot/invoke-native.ps1"

# 快速门禁只检查格式，不在验证阶段改写工作区.
$unformatted = Invoke-NativeCommand -Command { gofmt -l cmd internal types pkg } -Description "gofmt check"
if ($unformatted) {
    throw "Go files require gofmt:`n$unformatted"
}

Invoke-NativeCommand -Command { go mod tidy -diff } -Description "go mod tidy -diff"
Invoke-NativeCommand -Command { go build ./... } -Description "go build"
Invoke-NativeCommand -Command { go test -count=1 ./... } -Description "go test"
Invoke-NativeCommand -Command { go test -race -count=1 ./... } -Description "go test -race"
Invoke-NativeCommand -Command { go vet ./... } -Description "go vet"
Invoke-NativeCommand -Command { git diff --check } -Description "git diff --check"
