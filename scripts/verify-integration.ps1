# 集成门禁装配真实 HTTP Listener 与 SQLite 文件，必须与快速单元门禁分别报告.
$ErrorActionPreference = "Stop"

go test -tags=integration -count=1 ./internal/bootstrap -run '^TestBackend'
go test -tags=integration -race -count=1 ./internal/bootstrap -run '^TestBackend'
