GO_PACKAGES = $(shell go list ./... | grep -v '/frontend/node_modules/')
GO_LINT_PATHS = $(shell go list -f '{{.Dir}}' ./... | grep -v '/frontend/node_modules/')

# Local dependency paths (override with EKIT_PATH=... GIT_SDK_PATH=...)
EKIT_PATH ?= ../ekit
GIT_SDK_PATH ?= /opt/project/wechat_project/git-platform-sdk

.PHONY: help fmt lint lint-strict test test-race vuln frontend-install frontend-lint frontend-build compose-config check setup-local-deps

help:
	@echo "Targets:"
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

frontend-install:
	npm ci --prefix frontend

frontend-lint:
	npm run lint --prefix frontend

frontend-build:
	npm run build --prefix frontend

compose-config:
	docker compose config >/dev/null

check: test lint frontend-lint frontend-build

setup-local-deps:
	@echo "Checking local dependencies..."
	@if [ ! -d "$(EKIT_PATH)" ]; then \
		echo "WARNING: ekit not found at $(EKIT_PATH). Clone it: git clone <ekit-repo> $(EKIT_PATH)"; \
	else \
		echo "OK: ekit found at $(EKIT_PATH)"; \
	fi
	@if [ ! -d "$(GIT_SDK_PATH)" ]; then \
		echo "WARNING: git-platform-sdk not found at $(GIT_SDK_PATH). Clone it or override with GIT_SDK_PATH=..."; \
	else \
		echo "OK: git-platform-sdk found at $(GIT_SDK_PATH)"; \
	fi
	@echo ""
	@echo "Current go.mod replace directives:"
	@grep "^replace" go.mod || echo "  (none)"
