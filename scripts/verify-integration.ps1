# 集成门禁装配真实 HTTP Listener 与 SQLite 文件，必须与快速单元门禁分别报告.
$ErrorActionPreference = "Stop"
. "$PSScriptRoot/invoke-native.ps1"

Invoke-NativeCommand -Command { go test -tags=integration -count=1 ./internal/bootstrap -run '^TestBackend' } -Description "backend integration test"
Invoke-NativeCommand -Command { go test -tags=integration -race -count=1 ./internal/bootstrap -run '^TestBackend' } -Description "backend integration race test"
