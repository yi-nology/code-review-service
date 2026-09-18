# Review Service API Documentation

## Overview

The Review Service provides a multi-agent code review platform with real-time progress tracking, agent discussions, requirements-aware reviews, and code review maps.

## Base URL

```
http://localhost:{port}/api/v1
```

## Authentication

All API endpoints require Bearer token authentication:

```
Authorization: Bearer <your-token>
```

---

## Endpoints

### Reviews

#### Create Review
```http
POST /reviews
Content-Type: application/json

{
  "provider": "github",
  "repo": "owner/repo",
  "pull_request_id": "123",
  "base_sha": "abc123",
  "head_sha": "def456",
  "author": "username",
  "author_name": "Display Name",
  "author_avatar": "https://...",
  "callback_mode": "comment",
  "engine": "ocr"
}
```

`engine` 审查引擎（可选）：
- `ocr`（默认）——本服务多智能体 OCR 子进程工作流，全输入形态（PR/分支对比/单 commit）；
- `argus`——派发外部 Argus Bot（`POST {argus.bot_url}/tasks`），**仅支持 PR/MR 维度**（需
  `pull_request_id`/`merge_request_iid`，repo 须为 `owner/repo`，不支持 `commit_sha` 与同步
  `wait`），**仅支持 gitea/github/gitlab 平台**（其余平台创建即被拒，400）；异步回报：Argus
  完成后 push 到 `/api/v1/argus/reviews`，服务端自动把对应 active 评审收敛到终态并向 PR head
  写门禁 commit status，红绿按项目策略 `gate_mode` 分流（见下）；触发失败仅 strong 模式写
  error status。Argus 任务失败不回推，由 30s 兜底轮询收敛，回推丢失时自动拉取全量结果恢复。
- `task_kind`（可选，仅 `engine=argus`）：`code_review`（默认，多专家代码评审）|
  `requirement_compliance`（需求复核 R1-R5，`requirements` 作为验收清单注入）|
  `visual_check`（视觉检查，需 argus 侧启用对应插件）。
- 留空时按项目策略 `review_engine` 解析（项目策略页/设置 API 可配），仍空则 `ocr`。
- `argus` 需配置 `argus.bot_url`，否则创建即被拒（400）。

#### Get Review
```http
GET /reviews/:id
```

#### List Reviews
```http
GET /reviews?page=1&page_size=20&status=success&provider=github&repo=owner/repo
```

#### Rerun Review
```http
POST /reviews/:id/rerun
```

#### Cancel Review
```http
POST /reviews/:id/cancel
```

---

### Sessions

#### List Sessions for Review
```http
GET /reviews/:id/sessions
```

#### Create Session
```http
POST /reviews/:id/sessions
```

#### Get Session
```http
GET /reviews/:id/sessions/:sessionId
```

#### Get Active Session
```http
GET /reviews/:id/sessions/active
```

#### Update Session Phase
```http
PUT /reviews/:id/sessions/:sessionId/phase
Content-Type: application/json

{
  "phase": "reviews",
  "progress": 50.0
}
```

#### Update Session Status
```http
PUT /reviews/:id/sessions/:sessionId/status
Content-Type: application/json

{
  "status": "completed"
}
```

#### Update Session Agent Results
```http
PUT /reviews/:id/sessions/:sessionId/agent-results
Content-Type: application/json

[
  {
    "agent_name": "frontend",
    "findings": [...],
    "summary": "...",
    "confidence": 0.85
  }
]
```

#### Update Session Discussion
```http
PUT /reviews/:id/sessions/:sessionId/discussion
Content-Type: application/json

{
  "id": "discussion-123",
  "round": 1,
  "entries": [...],
  "summary": "..."
}
```

#### Update Session Final Report
```http
PUT /reviews/:id/sessions/:sessionId/final-report
Content-Type: application/json

{
  "verdict": "APPROVE",
  "summary": "...",
  "score": 85,
  "gate_passed": true
}
```

#### Complete Session
```http
POST /reviews/:id/sessions/:sessionId/complete
Content-Type: application/json

{
  "verdict": "APPROVE",
  "summary": "Review completed successfully",
  "score": 85,
  "gate_passed": true
}
```

#### Fail Session
```http
POST /reviews/:id/sessions/:sessionId/fail
Content-Type: application/json

{
  "reason": "OCR execution failed"
}
```

#### Cancel Session
```http
POST /reviews/:id/sessions/:sessionId/cancel
```

#### Get Session Map
```http
GET /reviews/:id/sessions/:sessionId/map
```

---

### Progress Tracking

#### WebSocket Connection
```http
WS /reviews/:id/progress
```

Connect to receive real-time progress updates via WebSocket.

#### Get Progress (REST)
```http
GET /reviews/:id/progress/status
```

#### Get Active Reviews
```http
GET /progress/active
```

---

### Review Maps

#### Generate Map
```http
POST /reviews/:id/maps
Content-Type: application/json

{
  "session_id": "session-123",
  "diff": "... unified diff ..."
}
```

#### List Maps
```http
GET /reviews/:id/maps
```

#### Get Map
```http
GET /reviews/:id/maps/:mapId
```

#### Preview Map (without saving)
```http
POST /reviews/:id/maps/preview
Content-Type: application/json

{
  "diff": "... unified diff ..."
}
```

---

### Publishing

#### Publish Review to PR/MR
```http
POST /reviews/:id/publish
```

#### Preview Comment
```http
GET /reviews/:id/preview
```

#### Update Published Comment
```http
PUT /reviews/:id/publish
```

---

### Settings - OCR

#### Get OCR Settings
```http
GET /settings/ocr
```

#### Update OCR Settings
```http
PUT /settings/ocr
Content-Type: application/json

{
  "settings": {
    "binary": "ocr",
    "url": "https://api.openai.com/v1",
    "model": "gpt-4",
    "auth_token": "sk-...",
    "language": "zh"
  }
}
```

#### Test LLM Connection
```http
POST /settings/ocr/test-llm
```

---

### Settings - Agents

#### List Agents
```http
GET /settings/agents
```

#### Create Agent
```http
POST /settings/agents
Content-Type: application/json

{
  "name": "frontend",
  "display_name": "前端审查员",
  "description": "Reviews frontend code",
  "background": "重点关注前端代码质量：Vue 组件设计、CSS 样式规范、响应式设计",
  "enabled": true,
  "priority": 1,
  "tags": ["frontend", "vue", "css"]
}
```

#### Update Agent
```http
PUT /settings/agents/:id
Content-Type: application/json

{
  "display_name": "Updated Name",
  "background": "Updated background text",
  "enabled": false,
  "priority": 2
}
```

#### Delete Agent
```http
DELETE /settings/agents/:id
```

---

### Settings - Projects

#### List Project Policies
```http
GET /settings/projects
```

#### Update Project Policy
```http
PUT /settings/projects
Content-Type: application/json

{
  "policy": {
    "provider": "github",
    "repo": "owner/repo",
    "fail_on": "critical",
    "gate_mode": "weak",
    "ignore_paths": ["vendor/**", "**/*.pb.go"],
    "rules": [
      {
        "path": "**/*.go",
        "rule": "Follow Go best practices"
      }
    ]
  }
}
```

`gate_mode` PR 门禁强弱依赖模式（可选，缺省 auto）：
- `""`（auto）——按引擎默认：OCR 强门禁、argus 弱门禁；
- `strong`——强依赖（事实门禁）：评审未过写 `failed`、执行失败写 `error`，可在分支保护里
  配为必需检查；
- `weak`——弱依赖：任何终态一律 `success` 留痕（裁决/失败细节放 description，带
  「【弱门禁】」前缀），不产出红 status，不会卡「所有检查通过后合并」类自动合并；
  argus 触发失败不发 status（仅评审记录与应用内通知）。
门禁红绿只影响 PR 上的 commit status；评审记录内的门禁裁决与应用内通知不受影响。

#### Delete Project Policy
```http
DELETE /settings/projects?provider=github&repo=owner/repo
```

---

### Settings - Project Agents

#### Get Project Agent Configuration
```http
GET /settings/projects/:provider/:repo/agents
```

Response:
```json
{
  "provider": "github",
  "repo": "owner/repo",
  "agents": [
    {
      "ID": 1,
      "name": "frontend",
      "display_name": "前端审查员",
      "background": "...",
      "enabled": true,
      "priority": 1,
      "project_enabled": true,
      "project_priority": 10
    },
    {
      "ID": 2,
      "name": "security",
      "display_name": "安全审查员",
      "background": "...",
      "enabled": true,
      "priority": 2,
      "project_enabled": false,
      "project_priority": 0
    }
  ]
}
```

#### Update Project Agent
```http
PUT /settings/projects/:provider/:repo/agents
Content-Type: application/json

{
  "agent_id": 1,
  "enabled": true,
  "priority": 10
}
```

#### Batch Update Project Agents
```http
POST /settings/projects/:provider/:repo/agents/batch
Content-Type: application/json

{
  "agents": [
    { "agent_id": 1, "enabled": true, "priority": 10 },
    { "agent_id": 2, "enabled": true, "priority": 20 },
    { "agent_id": 3, "enabled": false, "priority": 0 }
  ]
}
```

#### Delete Project Agent
```http
DELETE /settings/projects/:provider/:repo/agents/:agentId
```

---

### Settings - Git Platforms

#### List Platforms
```http
GET /settings/platforms
```

#### Add/Update Platform
```http
PUT /settings/platforms
Content-Type: application/json

{
  "provider": "github",
  "base_url": "https://github.com",
  "token": "ghp_...",
  "enabled": true
}
```

#### Delete Platform
```http
DELETE /settings/platforms?provider=github
```

#### Test Platform Connection
```http
POST /settings/platforms/test
Content-Type: application/json

{
  "provider": "github",
  "base_url": "https://github.com",
  "token": "ghp_..."
}
```

---

### Settings - Global Rules

#### Get Global Rules
```http
GET /settings/rules/global
```

#### Update Global Rules
```http
PUT /settings/rules/global
Content-Type: application/json

{
  "rules": {
    "rules": [
      "Follow Google Go Style Guide",
      "All exported functions must have comments"
    ]
  }
}
```

---

### Platforms (Provider Integration)

#### List Repos
```http
GET /platforms/:provider/repos?page=1&per_page=30
```

#### List Branches
```http
GET /platforms/:provider/repos/:owner/:repo/branches
```

#### Get Branch SHA
```http
GET /platforms/:provider/repos/:owner/:repo/branch-sha?branch=main
```

#### List Pull/Merge Requests
```http
GET /platforms/:provider/repos/:owner/:repo/pulls?state=opened
```

---

### Statistics

#### List Contributors
```http
GET /stats/contributors?page=1&page_size=20
```

#### Get Contributor Details
```http
GET /stats/contributors/:author
```

#### Get Contributor Trend
```http
GET /stats/contributors/:author/trend
```

#### Get Contributor Categories
```http
GET /stats/contributors/:author/categories
```

---

### Webhooks

#### Handle Webhook (GitHub/GitLab/Gitea)
```http
POST /webhooks/:provider/:installation
Content-Type: application/json
X-GitHub-Event: pull_request
X-GitHub-Delivery: xxx
X-Hub-Signature-256: sha256=...

{ ... webhook payload ... }
```

---

### Health Check

```http
GET /health
GET /ping
```

---

## Response Formats

### Success Response
```json
{
  "data": { ... },
  "message": "success"
}
```

### Error Response
```json
{
  "error": "error message",
  "details": { ... }
}
```

### Pagination
```json
{
  "items": [...],
  "total": 100,
  "page": 1,
  "page_size": 20,
  "total_pages": 5
}
```

---

## WebSocket Messages

### Progress Event
```json
{
  "type": "phase_progress",
  "phase": "reviews",
  "status": "active",
  "progress": 50.0,
  "message": "Running review agents",
  "agent_status": {
    "frontend": {
      "name": "frontend",
      "status": "running",
      "progress": 75.0,
      "message": "Analyzing Vue components"
    },
    "security": {
      "name": "security",
      "status": "completed",
      "progress": 100.0,
      "message": "Found 3 security issues"
    }
  },
  "timestamp": "2026-06-14T12:00:00Z"
}
```

### Event Types
| Event | Description |
|-------|-------------|
| `phase_start` | Phase started |
| `phase_progress` | Phase progress update |
| `phase_complete` | Phase completed |
| `agent_status` | Agent status update |
| `error` | Error occurred |
| `complete` | Review completed |

### Phases
| Phase | Description |
|-------|-------------|
| `context` | Context Discovery |
| `change_context` | Change Analysis |
| `analysis` | Tech Lead Analysis |
| `reviews` | Parallel Reviews |
| `aggregation` | Aggregation |
| `discourse` | Discussion |
| `synthesis` | Synthesis |
| `complete` | Complete |

---

## Rate Limiting

- **Global**: 100 requests/second, burst 200
- **Per-user**: 10 requests/second, burst 20

When rate limited:
```http
HTTP/1.1 429 Too Many Requests
Retry-After: 1

{
  "error": "rate limit exceeded",
  "retry_after": "1"
}
```

---

## Error Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 201 | Created |
| 202 | Accepted (async operation) |
| 204 | No Content |
| 400 | Bad Request |
| 401 | Unauthorized |
| 404 | Not Found |
| 409 | Conflict (duplicate) |
| 413 | Payload Too Large |
| 429 | Too Many Requests |
| 500 | Internal Server Error |

---

## Examples

### Complete Review Flow

1. **Create Review**
```bash
curl -X POST http://localhost:8080/api/v1/reviews \
  -H "Authorization: Bearer token" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "github",
    "repo": "owner/repo",
    "pull_request_id": "42",
    "base_sha": "abc123",
    "head_sha": "def456"
  }'
```

2. **Connect WebSocket for Progress**
```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/reviews/review-xxx/progress');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(`Phase: ${data.phase}, Progress: ${data.progress}%`);
};
```

3. **Get Results**
```bash
curl http://localhost:8080/api/v1/reviews/review-xxx \
  -H "Authorization: Bearer token"
```

4. **Get Review Map**
```bash
curl http://localhost:8080/api/v1/reviews/review-xxx/maps \
  -H "Authorization: Bearer token"
```

5. **Publish to PR**
```bash
curl -X POST http://localhost:8080/api/v1/reviews/review-xxx/publish \
  -H "Authorization: Bearer token"
```

---

## Argus Reviews (承接 Argus 审查结果)

Argus（PR 自动审查 Bot）以 push 模型上报审查结果；本组端点供其落库与查询。
鉴权同 `/api/v1/*`（Bearer token）。幂等：业务键 `(platform, owner, repo, number, head_sha)`
upsert，同 PR 同 SHA 重审覆盖；意见整组替换，非空 fp 上 `(review, fp)` 唯一
（同 payload 重复 fp 由承接侧 last-wins 去重；已抑制意见的 `suppress_state`
在重报同 SHA 时按 fp 继承，不被替换清空）。

**双向流程**：除 Argus 自主触发的 push 外，`POST /reviews` 携带 `engine=argus`
（或项目策略 `review_engine=argus`）时，本服务主动调 Argus `POST {argus.bot_url}/tasks`
创建 CODE_REVIEW 任务（Argus 侧同 PR 在飞任务幂等去重），评审 Record 停在 running；
Argus 完成后 POST 本端点落库，服务端按 `(platform, owner/repo, number, head_sha)` 把
engine=argus 的 active 评审自动收敛到终态（dispatch/argus_report 事件全程留痕）。
Argus 迟迟不回报时由既有 stale 重排兜底（重派发命中 Argus 幂等去重，不重复审查）。

### Ingest Argus Review

```http
POST /argus/reviews
Content-Type: application/json

{
  "argus_task_id": "task-123",
  "platform": "gitea",
  "owner": "owner",
  "repo": "repo",
  "number": "7",
  "title": "PR 标题",
  "head_sha": "abc123",
  "base_branch": "main",
  "verdict": "approve|comment|request_changes|error",
  "error_message": "verdict=error 时的失败原因（可选）",
  "findings": [
    {"fp": "指纹", "file": "a.go", "line": 3, "severity": "high|medium|low",
     "comment": "意见", "suggested_fix": "修复建议", "by": "来源插件"}
  ],
  "plugins": ["correctness", "ocr"],
  "duration_ms": 1500,
  "token_usage": 42,
  "occurred_at": "2026-09-16T12:00:00Z"
}
```

响应：`{"id": 1, "status": "accepted"}`。单次意见上限 500 条。

### List Argus Reviews

```http
GET /argus/reviews?repo=owner/repo&platform=gitea&severity=high&since=2026-09-01T00:00:00Z&page=1&page_size=20
```

响应：`{"items": [ArgusReview...], "total": N}`（按 occurred_at 降序；severity 过滤语义为"含该档意见"；
`platform` 精确匹配，多平台同名 repo 区分用）。

### Get Argus Review Detail

```http
GET /argus/reviews/:id
```

响应：`{"review": {..., "findings": [...]}}`（意见按 file/line 排序）。

### Suppress Argus Finding (反馈回流)

```http
POST /argus/reviews/:id/findings/suppress
Content-Type: application/json

{"fp": "argus 规范指纹"}
```

`fp` 与 `file+comment` 二选一（fp 优先）：fp 为 argus 规范指纹；无 fp 的旧数据或指纹
未命中（措辞漂移）时可用 `{"file": "path/to.go", "comment": "意见原文"}` 兜底，fp 模式
未命中且请求带 file+comment 时服务端也会自动降级尝试。

服务端代理调用 argus `POST /findings/suppress`（凭证只在服务端），命中后本地落
`suppress_state=suppressed`。响应：

- `{"matched": true, "state": "suppressed"}` —— argus 已置抑制，下一轮审查起自动剔除；
- `{"matched": false, "message": "..."}` —— argus 未命中（意见已被重审覆盖/措辞漂移）。

前置：配置 `argus.bot_url` + `argus.bot_token`（或环境变量 `ARGUS_BOT_URL`/`ARGUS_BOT_TOKEN`，
token 即 argus 侧 `ARGUS_APITOKEN`）；未配置时返回 400。

### Get Argus Review Report (报告全文)

```http
GET /argus/reviews/:id/report
```

懒拉取代理：按承接记录的 `argus_task_id` 调 argus `GET /tasks/:taskId` 返回报告全文
（markdown），本站不落库。响应：`{"argus_task_id": "...", "status": "done", "verdict":
"comment", "report": "# ...", "findings_count": 3}`。记录无任务 ID（纯上报/旧数据）返回 400。

### List Argus Tasks (任务对账)

```http
GET /argus/tasks?status=running&limit=50
```

argus 侧真实任务队列代理（`status` 空=全部，`limit` 上限 100）。响应：
`{"tasks": [{"task_id", "status", "kind", "platform", "owner", "repo", "number",
"head_sha", "verdict", "created_at"}]}`。

### Get Argus Usage (本月用量)

```http
GET /argus/usage
```

argus 本月各仓库 token 用量代理。响应：`{"month": "2026-09", "total": 1234, "budget":
5000, "repos": [{"platform", "owner", "repo", "tokens", "tasks"}]}`（budget=0 表示 argus
侧未设预算）。

### Get Argus Health (审查健康)

```http
GET /argus/health
```

argus 审查健康报告（学习环飞轮：噪声处置/指南建议/计数）代理。响应为
`{"payload": "<argus 侧原始 JSON 字符串>"}`，结构由 argus 侧定义（含
`generated_at`/`repos[]`/`empty`），前端解析渲染。

### Ask Argus (问答)

```http
POST /argus/ask
Content-Type: application/json

{"platform": "gitea", "owner": "o", "repo": "r", "number": "7", "question": "为什么这条是 high？"}
```

argus MCP `ask_argus` 工具的服务端代理（streamable http，凭证只在服务端）。
`question` 为空返回该 PR 最近审查结论摘要。响应：`{"answer": "..."}`；代理超时 90s。
