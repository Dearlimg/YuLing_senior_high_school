# 多阶段构建。
# 关键点：最终镜像里只有静态二进制 + CA 证书 + 时区数据，
# 不含 Go 工具链、源码、git 历史，体积从 ~1GB 降到 ~20MB，
# 同时消除了"镜像里翻出源码和 .git 导致泄露"的风险。

FROM golang:1.26-alpine AS build

WORKDIR /src

# GOPROXY 显式指定：构建机在国内，默认 proxy.golang.org 会超时卡死构建。
# CGO_ENABLED=0 产出纯静态二进制，才能在 alpine/scratch 里不依赖 glibc 运行。
ENV GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=0 \
    GOOS=linux

# 先只拷 go.mod/go.sum 再下载依赖，利用层缓存：
# 只改业务代码时不会重新拉取整个依赖树。
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# -trimpath 去掉构建机的绝对路径（避免泄露目录结构，也让构建可复现）
# -s -w 去掉符号表和 DWARF 调试信息
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server


FROM alpine:3.20

# ca-certificates：HTTPS 出站需要；tzdata：TZ=Asia/Shanghai 才真正生效。
# 缺 tzdata 时容器时间会静默退回 UTC，日志时间全错八小时。
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 10001 -S app \
    && adduser -u 10001 -S app -G app

ENV TZ=Asia/Shanghai

COPY --from=build /out/server /usr/local/bin/server

# 与 Deployment 中的 runAsUser 保持一致，避免"镜像里是 root、清单里声明非 root"的错配。
USER 10001:10001

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/server"]
