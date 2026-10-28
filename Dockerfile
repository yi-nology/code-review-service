# 全量构建（CI）：源码 → 二进制 → 运行时镜像。
#
# 多架构：buildx 传 --platform linux/amd64,linux/arm64 时
#   - 编译阶段固定 --platform=$BUILDPLATFORM —— 原生交叉编译，不走 QEMU 模拟（快）；
#   - 运行阶段按 TARGETARCH 取对应的 OCR 二进制（上游发布 amd64/arm64 两份）。
#   之前 GOARCH 与 OCR 二进制都写死 amd64：在 arm64 机器上 build 出来的镜像里
#   装的是 amd64 二进制，容器起来直接 exec format error。现在按目标架构取。
#
# 本地产物镜像用 Dockerfile.deploy（增量），本地构建用 Dockerfile.local。
# 三者的 OCR_VERSION 与 MCP npm 包清单必须保持一致。
#
# buildx 自动注入的平台参数：全局声明一次。
# 不给默认值 —— 平台参数一旦带了默认值而自动注入没生效，就会悄悄构建出
# 「架构对不上」的镜像（arm64 镜像里塞 amd64 二进制）。
ARG BUILDPLATFORM
ARG TARGETARCH

FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.26-alpine AS build

# 阶段内要用平台参数得再声明一次（不带值 = 继承全局的自动注入值）。
ARG TARGETARCH
# GOPROXY 可用 --build-arg 覆盖：默认 goproxy.cn（国内拉取快），
# GitHub runner 上传 proxy.golang.org 更稳（见 .github/workflows/docker.yml）。
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY} \
    GONOSUMCHECK='*' \
    GONOSUMDB='*' \
    GOTOOLCHAIN=auto
RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
# 预取模块带重试：代理 CDN 偶发传输层中断（stream error / INTERNAL_ERROR）——
# 那不是版本不存在，也不是校验失败，重试一次通常就过。
RUN set -eu; \
    ok=0; \
    for i in 1 2 3 4 5; do \
      if go mod download; then ok=1; break; fi; \
      echo "go mod download failed (attempt $i/5), retrying in 10s" >&2; \
      sleep 10; \
    done; \
    [ "$ok" = 1 ]
COPY . .
# 同时构建 review-service (主 HTTP 服务) 跟 mcp-server (MCP 端点, 独立进程)。
# 两个二进制用同一份代码,通过 fx wire 选 module 集合区分。
# ${TARGETARCH:-amd64}：拿不到平台参数时（老 buildx）按 amd64 兜底。
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -o /out/review-service . \
    && CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -o /out/mcp-server ./cmd/mcp-server

FROM docker.io/library/alpine:3.21

ARG TARGETARCH
RUN apk add --no-cache ca-certificates git wget curl \
    && adduser -D review

# Node.js（MCP server 运行时；apk nodejs 自带 npm/npx）
RUN apk add --no-cache nodejs npm

# OCR binary
ARG OCR_VERSION=v1.13.2
# OCR 上游只发布 linux-amd64 / linux-arm64：架构对不上就直接失败，
# 别把一个跑不起来的二进制装进镜像里等运行时才炸。
RUN set -eu; \
    arch="${TARGETARCH:-amd64}"; \
    case "${arch}" in \
      amd64|arm64) ocr_arch="${arch}" ;; \
      *) echo "unsupported TARGETARCH: ${arch} (OCR only ships amd64/arm64)" >&2; exit 1 ;; \
    esac; \
    wget -qO /usr/local/bin/ocr \
      "https://github.com/alibaba/open-code-review/releases/download/${OCR_VERSION}/opencodereview-linux-${ocr_arch}" \
    && chmod +x /usr/local/bin/ocr

# 预装 MCP server 包（与前端 mcp-templates.ts 一一对应）
# npx -y 首次调用时直接使用缓存，无需网络下载
RUN npm install -g \
    zfetch-mcp \
    @upstash/context7-mcp \
    @modelcontextprotocol/server-sequential-thinking \
    @modelcontextprotocol/server-memory \
    @modelcontextprotocol/server-time \
    2>/dev/null || true

WORKDIR /app
USER review
# review-service 路径跟历史保持一致(/usr/local/bin/review-service,默认 entrypoint)。
# mcp-server 放 /usr/local/bin/，compose 用 entrypoint override 启动。
# Apache-2.0 §4(a)：分发衍生作品时必须附许可证副本。
COPY --from=build /src/LICENSE /usr/local/share/licenses/review-service/LICENSE
COPY --from=build /out/review-service /usr/local/bin/review-service
COPY --from=build /out/mcp-server /usr/local/bin/mcp-server

EXPOSE 8080 8082
ENTRYPOINT ["review-service"]
