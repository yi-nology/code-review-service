FROM docker.io/library/golang:1.26-alpine AS build

ENV GOPROXY=https://goproxy.cn,direct \
    GONOSUMCHECK='*' \
    GONOSUMDB='*' \
    GOPRIVATE=''
RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/review-service .

FROM docker.io/library/alpine:3.21

RUN apk add --no-cache ca-certificates git \
    && adduser -D review
WORKDIR /app
USER review
COPY --from=build /out/review-service /usr/local/bin/review-service

EXPOSE 8080
ENTRYPOINT ["review-service"]
