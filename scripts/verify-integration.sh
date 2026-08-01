#!/usr/bin/env sh
# 集成门禁装配真实 HTTP Listener 与 SQLite 文件，并在 Unix 验证 SIGTERM。
set -eu

go test -tags=integration -count=1 ./internal/bootstrap -run '^TestBackend'
go test -tags=integration -race -count=1 ./internal/bootstrap -run '^TestBackend'
