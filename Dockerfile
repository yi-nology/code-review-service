FROM docker.io/library/golang:1.26-alpine AS build

ENV GOPROXY=https://goproxy.cn,direct
ENV GONOSUMCHECK='*'
ENV GONOSUMDB='*'
ENV GOPRIVATE=''
RUN apk add --no-cache git

WORKDIR /src
COPY review-service/go.mod review-service/go.sum ./
RUN go mod download
COPY review-service/main.go review-service/init.go ./
COPY review-service/biz ./biz
COPY review-service/internal ./internal
COPY review-service/swagger ./swagger
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/review-service .

# Build open-code-review (ocr) binary from source
WORKDIR /ocr-src
COPY ocr-build/go.mod ocr-build/go.sum ./
RUN go mod download
COPY ocr-build/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ocr ./cmd/opencodereview

FROM docker.io/library/alpine:3.21

RUN apk add --no-cache ca-certificates git \
    && adduser -D review
USER review
COPY --from=build /out/review-service /usr/local/bin/review-service
COPY --from=build /out/ocr /usr/local/bin/ocr

EXPOSE 8080
ENTRYPOINT ["review-service"]
