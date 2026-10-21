# Review Service / 代码评审服务

[English](README.md) | [中文](README.zh-CN.md)

## 概述

Review Service 是一个代码评审网关，支持 GitHub、GitLab、Gitea、Forgejo、Gitee、GitCode、腾讯工蜂和 Gerrit。它使用 AI 驱动的分析（通过 Open Code Review）自动审查 Pull Request 和 Merge Request，并将可操作的反馈直接发布到代码平台。

### 核心功能

- **多平台支持**：GitHub、GitLab、Gitea、Forgejo、Gitee、GitCode、腾讯工蜂、Gerrit
- **AI 驱动分析**：使用 Open Code Review (OCR) 进行智能代码评审
- **多 Agent 工作流**：并行评审 Agent，支持聚合和讨论
- **实时进度**：基于 WebSocket 的进度追踪
- **质量门禁**：可配置的严重级别阈值
- **行内评论**：评审结果作为行内评论写回 PR/MR
- **Web 控制台**：基于浏览器的评审管理界面
- **会话管理**：多轮评审会话，支持历史记录

### 生产与企业特性

- **CI 状态检查**：自动向 Git 平台写入 commit status（success/failed/error），使 review-service 可作为 PR 强制质量门禁
- **发现项生命周期**：跨评审稳定指纹识别复现问题；误报抑制避免重复上报已忽略的发现
- **通知**：应用内通知中心 + 外部 webhook（飞书 / 企业微信 / 钉钉），在评审完成/失败/门禁失败时触发
- **成本管理**：按模型的 LLM 成本估算价格表，支持月度预算上限与可配置的执行方式（warn/block）
- **可观测性**：Prometheus 指标（`/metrics` 端点），覆盖评审时长、队列深度、回写成功率与 LLM 成本；附可直接导入的 Grafana 看板（`deploy/grafana-dashboard.json`）
- **审计与鉴权**：具名 API Token（SHA256 哈希存储）与写操作审计日志，兼容单 Token 模式
- **可靠性**：持久化评审队列（PG `SKIP LOCKED`）、重试上限后的死信处理、不可重试错误分类（DNS 失败不会双重超时）
- **安全**：补丁路径穿越防护、敏感文件护栏（CI/构建/钩子）、自动修复推送的受保护分支强制约束
- **周报**：项目级评审汇总报告（Markdown）


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
│  │  Review  │ │ Workflow │ │  Agent   │ │ Webhook  │  │
│  │ Service  │ │  Engine  │ │ Manager  │ │ Service  │  │
│  │(Pipeline)│ └──────────┘ └──────────┘ └──────────┘  │
│  └──────────┘                                          │
├─────────────────────────────────────────────────────────┤
│              引擎适配层 (internal/eino)                   │
│         Execute + workflow / runner 两条路径              │
│         只持有 PipelineRunner 端口，不持仓储               │
├─────────────────────────────────────────────────────────┤
│                    领域层 (纯领域)                        │
│         internal/domain/review —— 类型与端口              │
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

> **分层方向：** 依赖单向流动——handler → service → domain ← 引擎适配层。
> 引擎适配层不持有仓储，只依赖 `review.PipelineRunner` 端口。这条由架构门禁
> （`architecture-policy.yaml`）强制执行。

> **部署说明：** 生产/Docker 环境下 Web 控制台由**独立的 nginx 容器**服务，nginx 把 `/api`、`/swagger`、`/healthz` 反向代理到 Go API 服务 —— Go 二进制只提供 API，不内嵌前端。本地开发可 `cd frontend && npm run dev`（Vite dev server 代理 `/api` 到后端）或 `make dev`。

## 快速开始

### 环境要求

- Go 1.27+
- PostgreSQL 14+
- Open Code Review 二进制文件（用于 AI 评审）

### 预编译二进制

每个 Release 都附带**静态链接**的 Linux 二进制——目标机器不需要装 Go 工具链
（无 glibc 依赖，Alpine 上也能直接跑）：

| 资产 | 架构 |
|---|---|
| `review-service-linux-amd64` | x86-64 |
| `review-service-linux-arm64` | aarch64 |
| `mcp-server-linux-amd64` | x86-64（MCP 端点） |
| `mcp-server-linux-arm64` | aarch64（MCP 端点） |

从 [Releases 页面](https://github.com/yi-nology/code-review-service/releases) 下载，
同时提供 `SHA256SUMS` 供校验：

```sh
sha256sum -c SHA256SUMS
./review-service-linux-amd64 --env prod
```

`Build Binaries` 工作流会在每次推送时重新构建，推 `v*` tag 时自动把二进制挂到
对应 Release。本地用 `make build-linux` 能以相同参数复现。

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

### 通知 / 成本 / 指标（可选）

这些项在 `etc/config-{env}.yaml` 中配置：

```yaml
notification:
  inapp_disabled: false          # 应用内通知（默认开启）
  webhooks:
    - type: feishu               # feishu / wecom / dingtalk / generic
      url: "${FEISHU_WEBHOOK_URL:}"
      on: ["failed", "gate_failed"]   # 触发条件

cost:
  monthly_budget_usd: 100.0      # 月度 LLM 成本预算（美元），0 = 不限
  review_action: "warn"          # warn（仅记录）/ block（拒绝新评审）
```

**Prometheus 指标**暴露在 `GET /metrics`（免认证）。`deploy/grafana-dashboard.json` 提供了可直接导入的看板。

**主要指标**：`review_service_review_total`、`review_service_review_duration_seconds`、`review_service_review_queue_depth`、`review_service_writeback_findings_total`、`review_service_llm_cost_usd_total`。

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
│   ├── argusclient/      # Argus Bot 客户端
│   ├── config/           # 配置加载与校验
│   ├── domain/           # 纯领域层（叶子，不依赖 infra）
│   │   └── review/       # 评审领域类型与端口（Request/Response/Result/Finding…）
│   ├── eino/             # 引擎适配层（只持 PipelineRunner 端口，不持仓储）
│   ├── mcp/              # MCP 服务端
│   ├── middleware/       # HTTP 中间件（认证 / 限流 / 安全头）
│   ├── modules/          # uber/fx 依赖注入模块
│   ├── observability/    # 可观测性（日志脱敏、redex 等）
│   ├── ocr/              # ★ OCR 核心包（所有 open-code-review 能力的统一入口）
│   ├── readiness/        # 健康就绪检查器（DB / OCR 二进制 / LLM endpoint）
│   ├── repository/       # 数据访问层接口和实现（postgres）
│   ├── strutil/          # 字符串工具
│   ├── vcs/              # Git 操作封装
│   └── service/          # 业务逻辑层
│       ├── agent/        # 多 Agent 管理
│       ├── discussion/   # Agent 讨论编排
│       ├── memory/       # 长期记忆（agent-memory 适配层）
│       ├── policy/       # 质量门禁策略
│       ├── progress/     # 进度追踪 + WebSocket 广播
│       ├── provider/     # Git 平台适配
│       ├── requirements/ # 需求解析
│       ├── review/       # 评审核心逻辑（Service / Worker / Pipeline / 报告构建）
│       ├── session/      # 会话管理
│       ├── stats/        # 统计分析
│       ├── webhook/      # Webhook 处理
│       ├── workflow/     # 多阶段工作流
│       └── writeback/    # 评论回写
└── swagger/              # Swagger UI
```

### `internal/ocr` 核心包

所有 open-code-review 能力的统一入口：

```
internal/ocr/
├── registry.go            # Registry（通过 Registry 聚合所有能力）
├── config.go              # Config + Fallback + BuildConfig（配置合并单一真相）
├── settings.go            # Settings 接口 + SettingsData + SettingsSnapshot
├── settings_adapter.go    # models.OCRSettingsRepository 适配器
├── llm_agentkit.go        # LLM Provider（agentkit 适配）
├── llm_factory.go         # LLM Provider 工厂
├── llm_types.go           # Provider / CompleteRequest / NoopProvider / Presets
├── llm_metrics.go         # Metrics / MetricsProvider
├── runner.go              # Runner（实现 domain.Runner/Preview/Scan）
├── runner_errors.go       # 错误分类
├── runner_parse.go        # 文本解析
├── scanner.go             # Preview + Scan 独立文件
├── delegate.go            # Delegator（任务派发）
├── managers.go            # 7 个 manager：Versioner/Lister/Checker/Viewer/Upgrader/SessionLister
├── tester.go              # Tester + ConfigResolver
├── factory.go             # Factory.CreateRunner
├── command.go             # OCR 命令封装
├── failure.go             # OCR 失败分类
├── env.go                 # AppendEnv / MergeLLMEnv
├── reasoning.go           # ResolveExtraBody 完整实现
├── helpers.go             # firstNonBlank / firstPositive
├── mcp_config.go          # MCP 配置
└── internal/              # exec 公共原语
    └── command.go
```

**关键设计：**
- `Config + Settings` 分离：`Config` 是合并后的运行时配置，`Settings` 是配置源抽象
- `BuildConfig` 是合并的单一实现（overrides > settings > fallback > defaults）
- `internal/ocr/internal/command.go` 的 `Run` 是统一的 exec 入口（三段式 SIGTERM → KillGrace → SIGKILL）
- `internal/domain/review/ocr_aliases.go` 正式定义 OCR 管理接口（LLMTester / LLMProviderLister / RulesChecker / OCRVersioner / ViewerManager / OCRUpgrader / SessionLister）

## 文档

- [`docs/API.md`](docs/API.md) — HTTP 接口参考
- [`docs/FUNCTIONAL_OVERVIEW.md`](docs/FUNCTIONAL_OVERVIEW.md) — 控制台功能介绍（附界面截图）
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
