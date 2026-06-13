FROM golang:1.26-alpine AS build

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go server.go init.go ./
COPY biz ./biz
COPY internal ./internal
COPY swagger ./swagger
RUN CGO_ENABLED=0 go build -o /out/review-service .

FROM alpine:3.22

ARG TARGETARCH
ARG OCR_VERSION=latest
RUN apk add --no-cache ca-certificates git \
    && wget -qO /usr/local/bin/ocr "https://github.com/alibaba/open-code-review/releases/${OCR_VERSION}/download/opencodereview-linux-${TARGETARCH}" \
    && chmod +x /usr/local/bin/ocr \
    && adduser -D review
USER review
COPY --from=build /out/review-service /usr/local/bin/review-service

EXPOSE 8080
ENTRYPOINT ["review-service"]
