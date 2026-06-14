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
  "callback_mode": "comment"
}
```

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
