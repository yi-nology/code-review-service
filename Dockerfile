FROM docker.io/library/golang:1.26-alpine AS build

ENV GOPROXY=https://goproxy.cn,direct \
    GONOSUMCHECK='*' \
    GONOSUMDB='*' \
    GOPRIVATE='' \
    # github.com/yi-nology 证书过期时仍可拉取 ekit（仅构建期,运行时不含源码）。
    GOINSECURE='' \
    GIT_SSL_NO_VERIFY='false' \
    GOTOOLCHAIN=auto
RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 同时构建 review-service (主 HTTP 服务) 跟 mcp-server (MCP 端点, 独立进程)。
# 两个二进制用同一份代码,通过 fx wire 选 module 集合区分。
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/review-service . \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/mcp-server ./cmd/mcp-server

FROM docker.io/library/alpine:3.21

RUN apk add --no-cache ca-certificates git wget curl \
    && adduser -D review

# Node.js（MCP server 运行时）
RUN apk add --no-cache nodejs npm \
    && npm install -g npx

# OCR binary
ARG OCR_VERSION=v1.11.5
RUN wget -qO /usr/local/bin/ocr \
    "https://github.com/alibaba/open-code-review/releases/download/${OCR_VERSION}/opencodereview-linux-amd64" \
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
COPY --from=build /out/review-service /usr/local/bin/review-service
COPY --from=build /out/mcp-server /usr/local/bin/mcp-server

EXPOSE 8080 8082
ENTRYPOINT ["review-service"]
