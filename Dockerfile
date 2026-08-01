# 构建阶段固定 Go 补丁版本，并关闭 VCS 路径嵌入以减少机器差异。
FROM golang:1.25.7-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY types ./types
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/micro-go ./cmd/app

# 默认进程不依赖 shell 或系统动态库；配置和非 root 身份是运行镜像的全部内容。
FROM scratch
WORKDIR /app
COPY --from=build /out/micro-go /app/micro-go
COPY config/app.yaml /app/config/app.yaml
USER 65532:65532
ENTRYPOINT ["/app/micro-go"]
