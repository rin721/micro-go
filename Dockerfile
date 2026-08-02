# 构建阶段固定 Go 补丁版本，并关闭 VCS 路径嵌入以减少机器差异。
FROM golang:1.25.7-alpine AS build

# 所有相对 COPY 和 go build 都在独立源码目录执行。
WORKDIR /src
# 先复制依赖清单并下载模块，源码变化时可以复用依赖缓存层。
COPY go.mod go.sum ./
RUN go mod download
# 只复制构建应用所需的代码根，不把测试文档和本地临时资产带入阶段。
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY types ./types
# 禁用 CGO 得到可放入 scratch 的静态二进制，并去除宿主路径和 VCS 信息。
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/micro-go ./cmd/app

# 默认进程不依赖 shell 或系统动态库；配置和非 root 身份是运行镜像的全部内容。
FROM scratch
# 相对配置路径以 /app 为基准，与默认 config/app.yaml 保持一致。
WORKDIR /app
# 运行阶段只复制构建产物和默认配置，不包含 Go 工具链或源码。
COPY --from=build /out/micro-go /app/micro-go
COPY config/app.yaml /app/config/app.yaml
# 使用无特权数字用户运行，scratch 不需要提供 passwd 文件。
USER 65532:65532
# 直接执行应用，让容器停止信号到达 Go 进程而不经过 shell。
ENTRYPOINT ["/app/micro-go"]
