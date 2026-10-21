# Review Service

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.27%2B-00ADD8.svg)](https://go.dev/)

[English](README.md) | [中文](README.zh-CN.md)

[![CI](https://github.com/yi-nology/code-review-service/actions/workflows/quality.yml/badge.svg)](https://github.com/yi-nology/code-review-service/actions/workflows/quality.yml)

---

## Overview

Review Service is a code review gateway that integrates with GitHub, GitLab, Gitea, GitCode, and Tencent Code. It uses AI-powered analysis (via Open Code Review) to automatically review pull requests and merge requests, providing actionable feedback directly on the code platform.

### Key Features

- **Multi-platform support**: GitHub, GitLab, Gitea, GitCode, Tencent Code
- **AI-powered analysis**: Uses Open Code Review (OCR) for intelligent code review
- **Multi-agent workflow**: Parallel review agents with aggregation and discussion
- **Real-time progress**: WebSocket-based progress tracking
- **Quality gates**: Configurable severity thresholds for pass/fail decisions
- **Inline comments**: Findings written back as inline PR/MR comments
- **Web console**: Browser-based UI for viewing and managing reviews
- **Session management**: Multi-round review sessions with history
- **Review maps**: Visual code architecture maps with Mermaid diagrams

### Production & Enterprise Features

- **CI Status Check**: Automatically writes commit status (success/failed/error) to Git platforms, enabling review-service as a required PR quality gate
- **Finding lifecycle**: Stable cross-review fingerprints identify recurring issues; false-positive suppression prevents re-reporting dismissed findings
- **Notifications**: In-app notification center + external webhooks (Feishu/Lark, WeCom, DingTalk) on review completion/failure/gate-fail
- **Cost management**: Per-model pricing table for LLM cost estimation, monthly budget limits with configurable enforcement (warn/block)
- **Observability**: Prometheus metrics (`/metrics` endpoint) for review duration, queue depth, writeback rate, and LLM cost; ready-to-import Grafana dashboard (`deploy/grafana-dashboard.json`)
- **Audit & auth**: Named API tokens (SHA256-hashed) with audit logging of write operations, backward-compatible with single-token mode
- **Reliability**: Persistent review queue (PG `SKIP LOCKED`), retry-limit dead-letter handling, non-retryable error classification (DNS failures won't double-timeout)
- **Security**: Patch path-traversal protection, sensitive-file guardrails (CI/build/hooks), protected-branch enforcement for auto-fix pushes
- **Weekly reports**: Project-level review summary reports in Markdown

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Web Console (Vue.js)                  │
├─────────────────────────────────────────────────────────┤
│                    HTTP API (Hertz)                      │
├──────────┬──────────┬──────────┬──────────┬─────────────┤
│  Review  │ Settings │ Session  │ Progress │   Stats     │
│ Handler  │ Handler  │ Handler  │ Handler  │  Handler    │
├──────────┴──────────┴──────────┴──────────┴─────────────┤
│                   Service Layer (fx)                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │  Review  │ │ Workflow │ │  Agent   │ │ Webhook  │  │
│  │ Service  │ │  Engine  │ │ Manager  │ │ Service  │  │
│  │ (Pipeline)│ └──────────┘ └──────────┘ └──────────┘  │
│  └──────────┘                                          │
├─────────────────────────────────────────────────────────┤
│              Engine Adapter Layer (internal/eino)       │
│         Execute + workflow/runner paths                 │
│         holds only the PipelineRunner port              │
├─────────────────────────────────────────────────────────┤
│                  Domain Layer (pure)                    │
│        internal/domain/review — types & ports           │
├─────────────────────────────────────────────────────────┤
│                  Repository Layer (GORM)                 │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │  Review  │ │ Session  │ │ Settings │ │  Stats   │  │
│  │   Repo   │ │   Repo   │ │   Repo   │ │   Repo   │  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │
├─────────────────────────────────────────────────────────┤
│              PostgreSQL + Git Platform SDK               │
└─────────────────────────────────────────────────────────┘
```

> **Layering:** dependencies flow one way — handlers → services → domain ← engine adapter.
> The engine adapter holds no repository; it depends only on the `review.PipelineRunner`
> port. This is enforced by the architecture gate (`architecture-policy.yaml`).

> **Deployment note:** In production/Docker the web console is served by a **separate nginx container** that reverse-proxies `/api`, `/swagger`, `/healthz` to the Go API service — the Go binary serves only the API, it does not embed the frontend. For local development run `cd frontend && npm run dev` (Vite dev server proxies `/api` to the backend) or `make dev`.

## Quick Start

### Prerequisites

- Go 1.27+
- PostgreSQL 14+
- Open Code Review binary (for AI-powered reviews)

### Prebuilt binaries

Every release ships statically linked Linux binaries — no Go toolchain needed on
the target machine (glibc-free, so they also run on Alpine):

| Asset | Architecture |
|---|---|
| `review-service-linux-amd64` | x86-64 |
| `review-service-linux-arm64` | aarch64 |
| `mcp-server-linux-amd64` | x86-64 (MCP endpoint) |
| `mcp-server-linux-arm64` | aarch64 (MCP endpoint) |

Download them from the [Releases page](https://github.com/yi-nology/code-review-service/releases)
together with `SHA256SUMS` for verification:

```sh
sha256sum -c SHA256SUMS
./review-service-linux-amd64 --env prod
```

The `Build Binaries` workflow rebuilds them on every push and attaches them to
the matching release when a `v*` tag is pushed. To build them locally with the
same parameters, run `make build-linux`.

### Database setup

The service uses **GORM AutoMigrate** — on startup it creates all required tables automatically (enabled by default; set `DATABASE_AUTO_MIGRATE=false` to disable). You only need to **pre-create the database** itself:

```sh
psql -U postgres -c "CREATE DATABASE review_service;"
```

The committed `migrations/*.sql` files are optional manual migrations for specific column/index additions already covered by AutoMigrate; they are not run automatically. The `deploy/migrations/` files are one-off manual cleanup (e.g. dropping a removed feature's table) — run them by hand if upgrading an existing deployment.

### Run

```sh
DATABASE_URL='postgres://review_service:review_service@localhost:5432/review_service?sslmode=disable' \
REVIEW_TOKEN=dev-token \
go run .
```

The local profile (`etc/config-local.yaml`) listens on **:8081**. Docker Compose overrides `HTTP_ADDR=:8080`. See the env var table below.

### With OCR enabled

OCR runtime settings (binary path, LLM model, language, concurrency, etc.) are stored in the database. Enable and configure them from the web console (**Settings → Review Engine**) or the settings API after first launch. No environment variables are needed.

```sh
DATABASE_URL='postgres://review_service:review_service@localhost:5432/review_service?sslmode=disable' \
REVIEW_TOKEN=dev-token \
go run .
```

### Docker Compose

```sh
docker compose up --build
```

The compose stack starts PostgreSQL, the API service, and the web console. The API is exposed on `http://localhost:8080`, and the web console is exposed on `http://localhost/`. Git platform tokens, OCR settings, and project policies are all managed from the web console after launch — no environment variables needed.

### Health check

```sh
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
```

### Web Console

```sh
cd frontend && npm install && npm run dev
```

Open `http://localhost:3000/` and connect with `REVIEW_TOKEN`.

## API Reference

### Create Review

```sh
curl -X POST http://localhost:8081/api/v1/reviews \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "github",
    "repo": "owner/repo",
    "pull_request_id": "42",
    "callback_mode": "comment",
    "wait": false
  }'
```

### Get Review

```sh
curl http://localhost:8081/api/v1/reviews/{review_id} \
  -H "Authorization: Bearer dev-token"
```

### List Reviews

```sh
curl "http://localhost:8081/api/v1/reviews?provider=github&status=success&page=1&page_size=50" \
  -H "Authorization: Bearer dev-token"
```

### Webhook Endpoint

```sh
curl -X POST http://localhost:8081/api/v1/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "github",
    "installation": "install-123",
    "body": "<raw webhook payload>"
  }'
```

## Configuration

Business settings (Git platforms, OCR engine, project policies, review rules) are managed from the web console and stored in the database. The environment variables below are for infrastructure and deployment tuning only:

| Env | Default | Description |
|-----|---------|-------------|
| `HTTP_ADDR` | `:8081` | HTTP listen address |
| `REVIEW_TOKEN` | - | Bearer token for API authentication |
| `DATABASE_URL` | - | PostgreSQL connection URL |
| `WORKER_CONCURRENCY` | `2` | Number of background review workers |
| `WORKER_POLL_INTERVAL` | `1s` | Worker poll interval |
| `WORKER_STALE_AFTER` | `15m` | Stale worker timeout |
| `FEISHU_WEBHOOK_URL` | - | Feishu/Lark webhook URL for review notifications |
| `WECOM_WEBHOOK_URL` | - | WeCom (企业微信) webhook URL for review notifications |

### Notification / Cost / Metrics (optional)

These are configured via `etc/config-{env}.yaml`:

```yaml
notification:
  inapp_disabled: false          # in-app notifications (default on)
  webhooks:
    - type: feishu               # feishu / wecom / dingtalk / generic
      url: "${FEISHU_WEBHOOK_URL:}"
      on: ["failed", "gate_failed"]   # trigger conditions

cost:
  monthly_budget_usd: 100.0      # monthly LLM cost budget (USD), 0 = unlimited
  review_action: "warn"          # warn (log only) / block (reject new reviews)
```

**Prometheus metrics** are exposed at `GET /metrics` (auth-exempt). See `deploy/grafana-dashboard.json` for a ready-to-import dashboard.

**Key metrics**: `review_service_review_total`, `review_service_review_duration_seconds`, `review_service_review_queue_depth`, `review_service_writeback_findings_total`, `review_service_llm_cost_usd_total`.

## Development

### Build

```sh
go build ./...
```

### Test

```sh
go test ./...
```

### Lint

```sh
golangci-lint run ./...
```

## Project Structure

```
├── biz/
│   ├── handler/          # HTTP handlers
│   ├── model/            # Thrift-generated data models
│   └── router/           # Route registration
├── frontend/             # Vue.js web console
├── idl/                  # Thrift IDL definitions
├── internal/
│   ├── app/              # Application-layer interfaces (used by handlers)
│   ├── argusclient/      # Argus Bot client
│   ├── config/           # Config loading and validation
│   ├── domain/           # Pure domain layer (leaf; no infra deps)
│   │   └── review/       # Review domain types & ports (Request/Response/Result/Finding…)
│   ├── eino/             # Engine adapter layer (holds only the PipelineRunner port)
│   ├── mcp/              # MCP server
│   ├── middleware/       # HTTP middleware (auth / rate limit / security headers)
│   ├── modules/          # uber/fx dependency-injection modules
│   ├── observability/    # Observability (log redaction, metrics)
│   ├── ocr/              # ★ OCR core package (single entry for all open-code-review capabilities)
│   ├── readiness/        # Readiness checks (DB / OCR binary / LLM endpoint)
│   ├── repository/       # Data-access interfaces and postgres implementation
│   ├── strutil/          # String utilities
│   ├── vcs/              # Git operations
│   └── service/          # Business logic
│       ├── agent/        # Multi-agent management
│       ├── discussion/   # Agent discussion orchestration
│       ├── memory/       # Long-term memory (agent-memory adapter)
│       ├── policy/       # Quality-gate policy
│       ├── progress/     # Progress tracking + WebSocket broadcast
│       ├── provider/     # Git platform adapters
│       ├── requirements/ # Requirement parsing
│       ├── review/       # Review core logic (Service / Worker / Pipeline / report building)
│       ├── session/      # Session management
│       ├── stats/        # Statistics
│       ├── webhook/      # Webhook handling
│       ├── workflow/     # Multi-stage workflow
│       └── writeback/    # Comment writeback
└── swagger/              # Swagger UI
```

### The `internal/ocr` core package

The single entry point for all open-code-review capabilities:

```
internal/ocr/
├── registry.go            # Registry (aggregates every capability)
├── config.go              # Config + Fallback + BuildConfig (single source of config merge)
├── settings.go            # Settings interface + SettingsData + SettingsSnapshot
├── settings_adapter.go    # models.OCRSettingsRepository adapter
├── llm_agentkit.go        # LLM provider (agentkit adapter)
├── llm_factory.go         # LLM provider factory
├── llm_types.go           # Provider / CompleteRequest / NoopProvider / Presets
├── llm_metrics.go         # Metrics / MetricsProvider
├── runner.go              # Runner (implements domain.Runner/Preview/Scan)
├── runner_errors.go       # Failure classification
├── runner_parse.go        # Output parsing
├── scanner.go             # Preview + Scan
├── delegate.go            # Delegator (task dispatch)
├── managers.go            # 7 managers: Versioner/Lister/Checker/Viewer/Upgrader/SessionLister
├── tester.go              # Tester + ConfigResolver
├── factory.go             # Factory.CreateRunner
├── command.go             # OCR command wrapper
├── failure.go             # OCR failure classification
├── env.go                 # AppendEnv / MergeLLMEnv
├── reasoning.go           # ResolveExtraBody
├── helpers.go             # firstNonBlank / firstPositive
├── mcp_config.go          # MCP configuration
└── internal/              # Shared exec primitives
    └── command.go
```

**Key design points:**
- `Config` and `Settings` are separate: `Config` is the merged runtime configuration,
  `Settings` is the configuration-source abstraction.
- `BuildConfig` is the single config-merge implementation
  (overrides > settings > fallback > defaults).
- `Run` in `internal/ocr/internal/command.go` is the one exec entry
  (three-stage SIGTERM → KillGrace → SIGKILL).
- `internal/domain/review/ocr_aliases.go` defines the OCR management interfaces
  (LLMTester / LLMProviderLister / RulesChecker / OCRVersioner / ViewerManager /
  OCRUpgrader / SessionLister).

## Documentation

- [`docs/API.md`](docs/API.md) — HTTP API reference
- [`docs/CODE_REVIEW_POLICY.md`](docs/CODE_REVIEW_POLICY.md) — review & severity policy
- [`docs/OCR_CORE_DESIGN.md`](docs/OCR_CORE_DESIGN.md) — OCR engine integration design
- [`docs/webhook-writeback-design.md`](docs/webhook-writeback-design.md) — webhook & writeback design
- [`docs/PERFORMANCE.md`](docs/PERFORMANCE.md) — performance notes
- [`docs/archive/`](docs/archive/) — historical planning documents

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) for
the development setup, the commit convention, and the quality gate the CI
enforces, and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for the community
expectations.

## Security

Please do not report vulnerabilities through public issues. See
[SECURITY.md](SECURITY.md) for the private reporting channel and the areas
that need a security-focused review.

## License

Review Service is released under the **Apache License 2.0**. See
[LICENSE](LICENSE) for the full text, and [CHANGELOG.md](CHANGELOG.md) for the
release history.
