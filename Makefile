GO_PACKAGES = $(shell go list ./... | grep -v '/frontend/node_modules/')
GO_LINT_PATHS = $(shell go list -f '{{.Dir}}' ./... | grep -v '/frontend/node_modules/')

# Local dependency paths (override with EKIT_PATH=... GIT_SDK_PATH=...)
EKIT_PATH ?= ../ekit
GIT_SDK_PATH ?= ../go-git-platform

.PHONY: help fmt lint lint-strict test test-race vuln build run dev frontend-install frontend-lint frontend-build compose-config check setup-local-deps

help:
	@echo "Targets:"
	@echo "  make build           Compile the Go binary into bin/"
	@echo "  make run             Run the backend locally (config-local profile)"
	@echo "  make dev             Build the frontend and run the backend"
	@echo "  make fmt             Run gofmt on Go sources"
	@echo "  make lint            Run required Go static checks"
	@echo "  make lint-strict     Run extended golangci-lint policy"
	@echo "  make test            Run Go unit tests"
	@echo "  make test-race       Run Go tests with the race detector"
	@echo "  make vuln            Run govulncheck"
	@echo "  make frontend-lint   Run Vue/TypeScript lint"
	@echo "  make frontend-build  Run Vue typecheck and production build"
	@echo "  make compose-config  Validate docker compose configuration"
	@echo "  make check           Run the default local quality gate"
	@echo "  make setup-local-deps Verify/configure local replace directives"

build:
	go build -o bin/review-service .

# build-linux 交叉编译 Linux 二进制（amd64 + arm64），产物落在 dist/。
#
# 与 .github/workflows/build.yml 用同一套参数（CGO_ENABLED=0、-trimpath、
# -ldflags="-s -w"）：本地构建的产物与 CI 发的产物一致，校验和也能对上。
build-linux:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/review-service-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/mcp-server-linux-amd64 ./cmd/mcp-server
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/review-service-linux-arm64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/mcp-server-linux-arm64 ./cmd/mcp-server
	@cd dist && sha256sum review-service-linux-* mcp-server-linux-* > SHA256SUMS
	@ls -lh dist/

run:
	go run .

dev: frontend-build
	go run .

fmt:
	gofmt -w $$(git ls-files '*.go')

lint:
	go vet $(GO_PACKAGES)

lint-strict: lint
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run $(GO_LINT_PATHS); \
	else \
		echo "golangci-lint not found; install it to run the extended Go lint policy"; \
	fi

test:
	go test $(GO_PACKAGES)

test-race:
	go test -race $(GO_PACKAGES)

vuln:
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck $(GO_PACKAGES); \
	else \
		go run golang.org/x/vuln/cmd/govulncheck@latest $(GO_PACKAGES); \
	fi

# architecture-check 执行 architecture-policy.yaml 的可执行面。
#
# 判据来源是底座 agent-harness/archcheck（机械判据，不在本仓另立一套）。
# policy 文件既是声明面也是执行面：违反即非零退出。
architecture-check:
	go run github.com/yi-nology/agent-harness/cmd/archcheck --report

frontend-install:
	npm ci --prefix frontend

frontend-lint:
	npm run lint --prefix frontend

frontend-build:
	npm run build --prefix frontend

compose-config:
	docker compose config >/dev/null

check: test lint architecture-check frontend-lint frontend-build

setup-local-deps:
	@echo "Checking local dependencies..."
	@if [ ! -d "$(EKIT_PATH)" ]; then \
		echo "WARNING: ekit not found at $(EKIT_PATH). Clone it: git clone <ekit-repo> $(EKIT_PATH)"; \
	else \
		echo "OK: ekit found at $(EKIT_PATH)"; \
	fi
	@if [ ! -d "$(GIT_SDK_PATH)" ]; then \
		echo "WARNING: go-git-platform not found at $(GIT_SDK_PATH). Clone it or override with GIT_SDK_PATH=..."; \
	else \
		echo "OK: go-git-platform found at $(GIT_SDK_PATH)"; \
	fi
	@echo ""
	@echo "Current go.mod replace directives:"
	@grep "^replace" go.mod || echo "  (none)"
