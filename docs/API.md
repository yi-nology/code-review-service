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
  "head_sha": "def456"
}
```

#### Get Review
```http
GET /reviews/:id
```

#### List Reviews
```http
GET /reviews?page=1&page_size=20&status=success&provider=github
```

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

### Progress Tracking

#### WebSocket Connection
```http
WS /reviews/:id/progress
```

Connect to receive real-time progress updates.

#### Get Progress (REST)
```http
GET /reviews/:id/progress/status
```

#### Get Active Reviews
```http
GET /progress/active
```

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

#### Preview Map
```http
POST /reviews/:id/maps/preview
Content-Type: application/json

{
  "diff": "... unified diff ..."
}
```

### Publishing

#### Publish Review
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

### Settings

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
  "display_name": "Frontend Reviewer",
  "description": "Reviews frontend code",
  "background": "Focus on Vue components, CSS, and UI/UX",
  "enabled": true,
  "priority": 1
}
```

#### Update Agent
```http
PUT /settings/agents/:id
Content-Type: application/json

{
  "display_name": "Updated Name",
  "enabled": false
}
```

#### Delete Agent
```http
DELETE /settings/agents/:id
```

### Statistics

#### List Contributors
```http
GET /stats/contributors?page=1&page_size=20
```

#### Get Contributor
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

### Health Check

```http
GET /health
GET /ping
```

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
    }
  },
  "timestamp": "2026-06-14T12:00:00Z"
}
```

### Event Types
- `phase_start` - Phase started
- `phase_progress` - Phase progress update
- `phase_complete` - Phase completed
- `agent_status` - Agent status update
- `error` - Error occurred
- `complete` - Review completed

## Rate Limiting

- Global: 100 requests/second, burst 200
- Per-user: 10 requests/second, burst 20

When rate limited, the API returns:
```http
HTTP/1.1 429 Too Many Requests
Retry-After: 1

{
  "error": "rate limit exceeded",
  "retry_after": "1"
}
```
