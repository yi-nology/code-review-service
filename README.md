# Review Service / 代码评审服务

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

## Quick Start

### Prerequisites

- Go 1.26+
- PostgreSQL 14+
- Open Code Review binary (for AI-powered reviews)

### Run

```sh
DATABASE_URL='postgres://review_service:review_service@localhost:5432/review_service?sslmode=disable' \
REVIEW_TOKEN=dev-token \
go run .
```

### With OCR enabled

```sh
OCR_ENABLED=true \
OCR_BINARY=ocr \
DATABASE_URL='postgres://review_service:review_service@localhost:5432/review_service?sslmode=disable' \
REVIEW_TOKEN=dev-token \
go run .
```

### Health check

```sh
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

### Web Console

```sh
cd frontend && npm install && npm run dev
```

Open `http://localhost:3000/` and connect with `REVIEW_TOKEN`.

## API Reference

### Create Review

```sh
curl -X POST http://localhost:8080/api/v1/reviews \
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
curl http://localhost:8080/api/v1/reviews/{review_id} \
  -H "Authorization: Bearer dev-token"
```

### List Reviews

```sh
curl "http://localhost:8080/api/v1/reviews?provider=github&status=success&page=1&page_size=50" \
  -H "Authorization: Bearer dev-token"
```

### Webhook Endpoint

```sh
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "github",
    "installation": "install-123",
    "body": "<raw webhook payload>"
  }'
```

## Configuration

| Env | Default | Description |
|-----|---------|-------------|
| `HTTP_ADDR` | `:8080` | HTTP listen address |
| `REVIEW_TOKEN` | - | Bearer token for API authentication |
| `DATABASE_URL` | - | PostgreSQL connection URL |
| `OCR_ENABLED` | `false` | Enable AI-powered code review |
| `OCR_BINARY` | `ocr` | Path to Open Code Review binary |
| `WORKER_CONCURRENCY` | `2` | Number of background review workers |
| `POLICY_FAIL_ON` | `critical` | Severity threshold for quality gate |
| `POLICY_MAX_COMMENTS` | `20` | Maximum inline comments per review |
| `GITHUB_TOKEN` | - | GitHub API token |
| `GITLAB_TOKEN` | - | GitLab API token |
| `GITEA_TOKEN` | - | Gitea API token |

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

## 快速开始

### 环境要求

- Go 1.26+
- PostgreSQL 14+
- Open Code Review 二进制文件（用于 AI 评审）

### 运行

```sh
DATABASE_URL='postgres://review_service:review_service@localhost:5432/review_service?sslmode=disable' \
REVIEW_TOKEN=dev-token \
go run .
```

### 启用 OCR

```sh
OCR_ENABLED=true \
OCR_BINARY=ocr \
DATABASE_URL='postgres://review_service:review_service@localhost:5432/review_service?sslmode=disable' \
REVIEW_TOKEN=dev-token \
go run .
```

### 健康检查

```sh
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

### Web 控制台

```sh
cd frontend && npm install && npm run dev
```

打开 `http://localhost:3000/`，使用 `REVIEW_TOKEN` 连接。

## API 接口

### 创建评审

```sh
curl -X POST http://localhost:8080/api/v1/reviews \
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
curl http://localhost:8080/api/v1/reviews/{review_id} \
  -H "Authorization: Bearer dev-token"
```

### 评审列表

```sh
curl "http://localhost:8080/api/v1/reviews?provider=github&status=success&page=1&page_size=50" \
  -H "Authorization: Bearer dev-token"
```

### Webhook 端点

```sh
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "github",
    "installation": "install-123",
    "body": "<原始 webhook payload>"
  }'
```

## 配置项

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `HTTP_ADDR` | `:8080` | HTTP 监听地址 |
| `REVIEW_TOKEN` | - | API 认证 Bearer Token |
| `DATABASE_URL` | - | PostgreSQL 连接 URL |
| `OCR_ENABLED` | `false` | 启用 AI 代码评审 |
| `OCR_BINARY` | `ocr` | Open Code Review 二进制路径 |
| `WORKER_CONCURRENCY` | `2` | 后台评审 Worker 数量 |
| `POLICY_FAIL_ON` | `critical` | 质量门禁严重级别阈值 |
| `POLICY_MAX_COMMENTS` | `20` | 每次评审最大行内评论数 |
| `GITHUB_TOKEN` | - | GitHub API Token |
| `GITLAB_TOKEN` | - | GitLab API Token |
| `GITEA_TOKEN` | - | Gitea API Token |

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
│   ├── config/           # 配置加载
│   ├── errors/           # 统一错误类型
│   ├── middleware/        # HTTP 中间件
│   ├── modules/          # uber/fx 依赖注入模块
│   ├── repository/       # 数据访问层接口和实现
│   └── service/          # 业务逻辑层
│       ├── agent/        # 多 Agent 管理
│       ├── discussion/   # Agent 讨论编排
│       ├── executor/     # 评审执行器
│       ├── ocr/          # OCR 子组件
│       ├── policy/       # 质量门禁策略
│       ├── progress/     # 进度追踪
│       ├── provider/     # Git 平台适配
│       ├── requirements/ # 需求解析
│       ├── review/       # 评审核心逻辑
│       ├── reviewmap/    # 评审地图生成
│       ├── runner/       # OCR 运行器
│       ├── session/      # 会话管理
│       ├── stats/        # 统计分析
│       ├── types/        # 共享类型
│       ├── webhook/      # Webhook 处理
│       ├── workflow/     # 多阶段工作流
│       └── writeback/    # 评论回写
└── swagger/              # Swagger UI
```
