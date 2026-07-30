#!/usr/bin/env sh
set -eu

unformatted="$(gofmt -l kernel internal capability adapter examples)"
if [ -n "$unformatted" ]; then
  echo "Go files require gofmt:" >&2
  echo "$unformatted" >&2
  exit 1
fi

go mod tidy
go build ./...
go test ./...
go test -race ./...
go vet ./...

