FROM docker.io/library/golang:1.26-alpine AS build

ENV GOPROXY=https://goproxy.cn,direct
RUN apk add --no-cache git

WORKDIR /src
COPY review-service/go.mod review-service/go.sum ./
COPY ekit/go.mod ekit/go.sum ./ekit/
COPY git-platform-sdk/go.mod git-platform-sdk/go.sum ./git-platform-sdk/
# Fix replace paths for Docker context
RUN sed -i \
    -e 's|replace github.com/yi-nology/ekit => ../ekit|replace github.com/yi-nology/ekit => ./ekit|' \
    -e 's|replace github.com/yi-nology/git-platform-sdk => /opt/project/wechat_project/git-platform-sdk|replace github.com/yi-nology/git-platform-sdk => ./git-platform-sdk|' \
    go.mod
RUN go mod download
COPY review-service/main.go review-service/init.go ./
COPY review-service/biz ./biz
COPY review-service/internal ./internal
COPY review-service/swagger ./swagger
COPY ekit ./ekit
COPY git-platform-sdk ./git-platform-sdk
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/review-service .

FROM docker.io/library/alpine:3.21

RUN apk add --no-cache ca-certificates git \
    && adduser -D review
USER review
COPY --from=build /out/review-service /usr/local/bin/review-service

EXPOSE 8080
ENTRYPOINT ["review-service"]
