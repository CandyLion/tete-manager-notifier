# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
RUN go env -w GOPROXY=https://goproxy.cn,direct
WORKDIR /app
# 先拷贝 mod 文件利用 Docker 缓存
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /tmbark-notifier ./cmd/app
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
# 设置时区为上海
ENV TZ=Asia/Shanghai
WORKDIR /root/
# 从编译阶段拷贝二进制文件
COPY --from=builder /tmbark-notifier .
CMD ["./tmbark-notifier"]
