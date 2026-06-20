FROM docker.io/library/golang:1.26-alpine AS build

ENV GOPROXY=https://goproxy.cn,direct
RUN apk add --no-cache git

WORKDIR /src
COPY review-service/go.mod review-service/go.sum ./
COPY ekit/go.mod ekit/go.sum ./ekit/
# Fix the replace path for Docker context
RUN sed -i 's|replace github.com/yi-nology/ekit => ../ekit|replace github.com/yi-nology/ekit => ./ekit|' go.mod
RUN go mod download
COPY review-service/main.go review-service/init.go ./
COPY review-service/biz ./biz
COPY review-service/internal ./internal
COPY review-service/swagger ./swagger
COPY ekit ./ekit
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/review-service .

FROM docker.io/library/alpine:3.21

RUN apk add --no-cache ca-certificates git \
    && adduser -D review
USER review
COPY --from=build /out/review-service /usr/local/bin/review-service

EXPOSE 8080
ENTRYPOINT ["review-service"]
