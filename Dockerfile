FROM docker.io/library/golang:1.26-alpine AS build

ENV GOPROXY=https://goproxy.cn,direct \
    GONOSUMCHECK='*' \
    GONOSUMDB='*' \
    GOPRIVATE=''
RUN apk add --no-cache git

WORKDIR /src
COPY review-service/ ./review-service/
COPY ocr-build/ ./ocr-build/

# Build review-service
WORKDIR /src/review-service
RUN go mod download && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/review-service .

# Build open-code-review (ocr)
WORKDIR /src/ocr-build
RUN go mod download && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ocr ./cmd/opencodereview

FROM docker.io/library/alpine:3.21

RUN apk add --no-cache ca-certificates git \
    && adduser -D review
WORKDIR /app
USER review
COPY --from=build /out/review-service /usr/local/bin/review-service
COPY --from=build /out/ocr /usr/local/bin/ocr

EXPOSE 8080
ENTRYPOINT ["review-service"]
