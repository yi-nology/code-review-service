# Review Service / 代码评审服务

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.27%2B-00ADD8.svg)](https://go.dev/)
[![CI](https://github.com/yi-nology/code-review-service/actions/workflows/quality.yml/badge.svg)](https://github.com/yi-nology/code-review-service/actions/workflows/quality.yml)

[English](#english) | [中文](#中文)

---

# English

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
│  │ Executor │ │ Workflow │ │  Agent   │ │ Webhook  │  │
│  │ Service  │ │ Engine   │ │ Manager  │ │ Service  │  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │
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

> **Deployment note:** In production/Docker the web console is served by a **separate nginx container** that reverse-proxies `/api`, `/swagger`, `/healthz` to the Go API service — the Go binary serves only the API, it does not embed the frontend. For local development run `cd frontend && npm run dev` (Vite dev server proxies `/api` to the backend) or `make dev`.

## Quick Start

### Prerequisites

- Go 1.26+
- PostgreSQL 14+
- Open Code Review binary (for AI-powered reviews)

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

---

# 中文

## 概述

Review Service 是一个代码评审网关，支持 GitHub、GitLab、Gitea、GitCode 和腾讯工蜂。它使用 AI 驱动的分析（通过 Open Code Review）自动审查 Pull Request 和 Merge Request，并将可操作的反馈直接发布到代码平台。

### 核心功能

- **多平台支持**：GitHub、GitLab、Gitea、GitCode、腾讯工蜂
- **AI 驱动分析**：使用 Open Code Review (OCR) 进行智能代码评审
- **多 Agent 工作流**：并行评审 Agent，支持聚合和讨论
- **实时进度**：基于 WebSocket 的进度追踪
- **质量门禁**：可配置的严重级别阈值
- **行内评论**：评审结果作为行内评论写回 PR/MR
- **Web 控制台**：基于浏览器的评审管理界面
- **会话管理**：多轮评审会话，支持历史记录
- **评审地图**：可视化代码架构图（Mermaid 图表）

## 架构设计

```
┌─────────────────────────────────────────────────────────┐
│                  Web 控制台 (Vue.js)                      │
├─────────────────────────────────────────────────────────┤
│                    HTTP API (Hertz)                      │
├──────────┬──────────┬──────────┬──────────┬─────────────┤
│  Review  │ Settings │ Session  │ Progress │   Stats     │
│ Handler  │ Handler  │ Handler  │ Handler  │  Handler    │
├──────────┴──────────┴──────────┴──────────┴─────────────┤
│                服务层 (uber/fx 依赖注入)                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │ Executor │ │ Workflow │ │  Agent   │ │ Webhook  │  │
│  │ Service  │ │ Engine   │ │ Manager  │ │ Service  │  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │
├─────────────────────────────────────────────────────────┤
│                   数据访问层 (GORM)                       │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │  Review  │ │ Session  │ │ Settings │ │  Stats   │  │
│  │   Repo   │ │   Repo   │ │   Repo   │ │   Repo   │  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │
├─────────────────────────────────────────────────────────┤
│              PostgreSQL + Git Platform SDK               │
└─────────────────────────────────────────────────────────┘
```

> **部署说明：** 生产/Docker 环境下 Web 控制台由**独立的 nginx 容器**服务，nginx 把 `/api`、`/swagger`、`/healthz` 反向代理到 Go API 服务 —— Go 二进制只提供 API，不内嵌前端。本地开发可 `cd frontend && npm run dev`（Vite dev server 代理 `/api` 到后端）或 `make dev`。

## 快速开始

### 环境要求

- Go 1.26+
- PostgreSQL 14+
- Open Code Review 二进制文件（用于 AI 评审）

### 数据库初始化

服务使用 **GORM AutoMigrate** —— 启动时自动创建所有需要的表（默认开启，设置 `DATABASE_AUTO_MIGRATE=false` 可关闭）。你只需**预先创建数据库**本身：

```sh
psql -U postgres -c "CREATE DATABASE review_service;"
```

`migrations/*.sql` 是可选的手动迁移（AutoMigrate 已覆盖其内容，不会自动执行）；`deploy/migrations/` 是一次性手动清理脚本（如删除已下线特性的表），升级既有部署时手动执行。

### 运行

```sh
DATABASE_URL='postgres://review_service:review_service@localhost:5432/review_service?sslmode=disable' \
REVIEW_TOKEN=dev-token \
go run .
```

本地配置（`etc/config-local.yaml`）监听 **:8081**；Docker Compose 通过 `HTTP_ADDR=:8080` 覆盖。详见下方环境变量表。

### 启用 OCR

OCR 运行时配置（二进制路径、LLM 模型、输出语言、并发数等）保存在数据库中，首次启动后在 Web 控制台（**设置 → 审查引擎**）或 settings API 中配置即可，无需环境变量。

```sh
DATABASE_URL='postgres://review_service:review_service@localhost:5432/review_service?sslmode=disable' \
REVIEW_TOKEN=dev-token \
go run .
```

### Docker Compose

```sh
docker compose up --build
```

Compose 会启动 PostgreSQL、API 服务和 Web 控制台。API 暴露在 `http://localhost:8080`，Web 控制台暴露在 `http://localhost/`。Git 平台 Token、OCR 设置、项目策略等业务配置均在 Web 控制台管理，无需环境变量。

### 健康检查

```sh
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
```

### Web 控制台

```sh
cd frontend && npm install && npm run dev
```

打开 `http://localhost:3000/`，使用 `REVIEW_TOKEN` 连接。

## API 接口

### 创建评审

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

### 查询评审

```sh
curl http://localhost:8081/api/v1/reviews/{review_id} \
  -H "Authorization: Bearer dev-token"
```

### 评审列表

```sh
curl "http://localhost:8081/api/v1/reviews?provider=github&status=success&page=1&page_size=50" \
  -H "Authorization: Bearer dev-token"
```

### Webhook 端点

```sh
curl -X POST http://localhost:8081/api/v1/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "github",
    "installation": "install-123",
    "body": "<原始 webhook payload>"
  }'
```

## 配置项

业务配置（Git 平台、OCR 引擎、项目策略、审查规则）在 Web 控制台管理，已持久化到数据库。下表仅列出基础设施与部署调优相关的环境变量：

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `HTTP_ADDR` | `:8081` | HTTP 监听地址（Docker Compose 覆盖为 `:8080`） |
| `REVIEW_TOKEN` | - | API 认证 Bearer Token |
| `DATABASE_URL` | - | PostgreSQL 连接 URL |
| `WORKER_CONCURRENCY` | `2` | 后台评审 Worker 数量 |
| `WORKER_POLL_INTERVAL` | `1s` | Worker 轮询间隔 |
| `WORKER_STALE_AFTER` | `15m` | Worker 超时判定 |

## 开发指南

### 构建

```sh
go build ./...
```

### 测试

```sh
go test ./...
```

### 代码检查

```sh
golangci-lint run ./...
```

## 项目结构

```
├── biz/
│   ├── handler/          # HTTP 处理器
│   ├── model/            # Thrift 生成的数据模型
│   └── router/           # 路由注册
├── frontend/             # Vue.js 前端
├── idl/                  # Thrift IDL 定义
├── internal/
│   ├── app/              # 应用层接口（handler 依赖的服务接口）
│   ├── config/           # 配置加载与校验
│   ├── middleware/        # HTTP 中间件（认证 / 限流 / 安全头）
│   ├── modules/          # uber/fx 依赖注入模块
│   ├── observability/    # 可观测性（日志脱敏、redex 等）
│   ├── ocr/              # ★ OCR 核心包（所有 open-code-review 能力的统一入口）
│   ├── readiness/        # 健康就绪检查器（DB / OCR 二进制 / LLM endpoint）
│   ├── repository/       # 数据访问层接口和实现（postgres）
│   ├── vcs/              # Git 操作封装
│   └── service/          # 业务逻辑层
│       ├── agent/        # 多 Agent 管理
│       ├── discussion/   # Agent 讨论编排
│       ├── executor/     # 评审执行器（单 runner / 工作流分流）
│       ├── policy/       # 质量门禁策略
│       ├── progress/     # 进度追踪 + WebSocket 广播
│       ├── provider/     # Git 平台适配
│       ├── requirements/ # 需求解析
│       ├── review/       # 评审核心逻辑（Service / Worker / 报告构建）
│       │   └── domain/   # 纯领域类型与接口（Request/Response/Result/Finding/...）
│       ├── scoring/      # 评分计算（纯函数）
│       ├── session/      # 会话管理
│       ├── stats/        # 统计分析
│       ├── types/        # 共享领域类型（Finding / Severity 等）
│       ├── webhook/      # Webhook 处理
│       ├── workflow/     # 多阶段工作流
│       └── writeback/    # 评论回写
└── swagger/              # Swagger UI
```

### `internal/ocr` 核心包

所有 open-code-review 能力的统一入口（阶段 A+10 完整化）：

```
internal/ocr/
├── client.go              # Client 主入口
├── config.go              # Config + Fallback + BuildConfig（配置合并单一真相）
├── settings.go            # Settings 接口 + SettingsData + SettingsSnapshot
├── settings_adapter.go    # models.OCRSettingsRepository 适配器
├── llm.go                 # LLMProvider + Adapter（OpenAI / Anthropic 协议）
├── llm_types.go           # Provider / CompleteRequest / NoopProvider / Presets
├── llm_metrics.go         # Metrics / MetricsProvider
├── runner.go              # Runner（实现 domain.Runner/Preview/Scan）
├── scanner.go             # Preview + Scan 独立文件
├── managers.go            # 7 个 manager：Versioner/Lister/Checker/Viewer/Upgrader
├── tester.go              # Tester + ConfigResolver
├── factory.go             # Factory.CreateRunner
├── command.go             # Run / CommandSpec / CommandResult（公共 exec 入口）
├── reasoning.go           # ResolveExtraBody 完整实现
├── helpers.go             # firstNonBlank / firstPositive
├── registry.go            # Registry（通过 Registry 聚合所有能力）
└── internal/              # exec 公共原语
    └── command.go
```

**关键设计：**
- `Config + Settings` 分离：`Config` 是合并后的运行时配置，`Settings` 是配置源抽象
- `BuildConfig` 是合并的单一实现（overrides > settings > fallback > defaults）
- `internal.Run(ctx, CommandSpec)` 是统一的 exec 入口（三段式 SIGTERM → KillGrace → SIGKILL）
- `domain/ocr_aliases.go` 正式定义 OCR 管理接口（LLMTester / LLMProviderLister / RulesChecker / OCRVersioner / ViewerManager / OCRUpgrader）

---

## 文档

- [`docs/API.md`](docs/API.md) — HTTP 接口参考
- [`docs/CODE_REVIEW_POLICY.md`](docs/CODE_REVIEW_POLICY.md) — 评审与严重度策略
- [`docs/OCR_CORE_DESIGN.md`](docs/OCR_CORE_DESIGN.md) — OCR 引擎接入设计
- [`docs/webhook-writeback-design.md`](docs/webhook-writeback-design.md) — webhook 与回写设计
- [`docs/PERFORMANCE.md`](docs/PERFORMANCE.md) — 性能说明
- [`docs/archive/`](docs/archive/) — 历史规划文档归档

## 贡献指南

欢迎参与贡献。请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md)（开发环境、提交约定、
CI 强制执行的质量门禁）与 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)（社区行为
准则）。

## 安全

请勿通过公开 issue 报告安全问题。私密上报渠道与需要安全专项评审的区域见
[SECURITY.md](SECURITY.md)。

## 许可协议

Review Service 以 **Apache License 2.0** 发布。完整条款见 [LICENSE](LICENSE)，
版本变更记录见 [CHANGELOG.md](CHANGELOG.md)。
