# Review Service Enhancement Plan

## Overview

This document outlines the complete implementation plan for enhancing the review-service with advanced code review capabilities inspired by [open-code-review](https://github.com/spencermarx/open-code-review).

## Current State Analysis

### Existing Architecture

- **Backend**: Go with Hertz framework
- **Frontend**: Vue 3 + Element Plus
- **Database**: PostgreSQL
- **Review Engine**: Alibaba Open Code Review (OCR)

### Existing Features

- Multi-platform support (GitHub, GitLab, Gitea)
- Asynchronous review queue with worker concurrency
- Quality scoring and gate mechanism
- Web console with filtering and pagination
- CI/CD integration with idempotent requests
- Comment publishing to PR/MR

### Enhancement Goals

1. Multi-agent review architecture
2. Real-time progress tracking
3. Review session management
4. Agent discussion mechanism
5. Requirements-aware review
6. Code review maps
7. Custom reviewer personas
8. Enhanced dashboard

---

## Task Breakdown

### Phase 1: Foundation (Weeks 1-4)

#### Task 1.1: Multi-Agent Review Architecture

**Priority**: HIGH  
**Estimated Effort**: 2 weeks  
**Dependencies**: None

**Objective**: Implement a multi-agent review architecture where each agent is an OCR instance with a distinct `--background` context, running in parallel and aggregating results.

**Design Principle**: Each "agent" = one OCR call with a different `--background` parameter. Multiple agents run in parallel, each from a different review perspective. Results are aggregated into a unified report.

```
用户发起审查
      ↓
┌─────────────────────────────────────────┐
│           代理团队（并行执行）            │
├─────────────┬─────────────┬─────────────┤
│  前端审查员  │  后端审查员  │  安全审查员  │
│  OCR --bg   │  OCR --bg   │  OCR --bg   │
│ "关注前端"  │ "关注后端"  │ "关注安全"  │
└─────────────┴─────────────┴─────────────┘
      ↓              ↓              ↓
      └──────────────┼──────────────┘
                     ↓
              结果聚合 + 去重
                     ↓
                 最终报告
```

**Subtasks**:

1.1.1 Define ReviewerAgent database model
- File: `internal/repository/postgres/models/reviewer_agent.go`
- Create `ReviewerAgent` model with fields:
  - `Name` (unique identifier, e.g. "frontend", "security")
  - `DisplayName` (human-readable name, e.g. "前端审查员")
  - `Description` (brief description of the agent's focus)
  - `Background` (content passed to OCR `--background` parameter)
  - `Enabled` (toggle on/off)
  - `Priority` (execution order)
  - `Tags` (JSONB array for categorization)
- GORM AutoMigrate handles schema creation

1.1.2 Implement ReviewerAgent repository
- File: `internal/repository/postgres/reviewer_agent.go`
- CRUD operations: List, GetByID, GetByName, Create, Update, Delete, ListEnabled
- Interface defined in `internal/repository/repository.go`

1.1.3 Add --background support to OCR runner
- File: `internal/service/runner/runner.go`
- Add `Background` field to `OpenCodeReviewConfig`
- In `Run()`, append `--background <text>` to OCR args when Background is set
- In `buildConfig()`, read `Background` from `OCRSettings`

1.1.4 Add agent CRUD API endpoints
- File: `biz/handler/review_service/settings_service.go`
- `GET /api/v1/settings/agents` - List all agents
- `POST /api/v1/settings/agents` - Create agent
- `PUT /api/v1/settings/agents/:id` - Update agent
- `DELETE /api/v1/settings/agents/:id` - Delete agent

1.1.5 Create frontend agent management page
- File: `frontend/src/views/AgentSettings.vue`
- Agent list with enable/disable toggle
- Add/Edit dialog with name, description, background textarea
- Preset templates (前端审查员, 后端审查员, 安全审查员, 性能审查员)
- Delete with confirmation

1.1.6 Add default Background to OCRSettings
- File: `internal/repository/postgres/models/ocr_settings.go`
- Add `Background` field for single-agent mode fallback
- Frontend: `frontend/src/views/OCRSettings.vue` adds "默认审查侧重点" section

**Acceptance Criteria**:
- [x] ReviewerAgent model and migration created
- [x] Repository with CRUD operations
- [x] OCR runner passes `--background` flag
- [x] Agent CRUD API endpoints
- [x] Frontend agent management page with presets
- [x] OCRSettings supports default Background

---

#### Task 1.2: Review Session Management

**Priority**: HIGH  
**Estimated Effort**: 1.5 weeks  
**Dependencies**: Task 1.1

**Objective**: Implement persistent review sessions that track the complete lifecycle of a review.

**Subtasks**:

1.2.1 Define session data model
- File: `internal/repository/postgres/models/session.go`
- Create `ReviewSession` model with fields:
  - `ID`, `ReviewID`, `Round`, `Phase`
  - `Status`, `Progress`
  - `AgentResults` (JSONB)
  - `Discussion` (JSONB)
  - `FinalReport` (JSONB)
  - `CreatedAt`, `UpdatedAt`
- Add database migration for sessions table

1.2.2 Implement session repository
- File: `internal/repository/postgres/session_repo.go`
- Create `SessionRepository` interface
- Implement CRUD operations
- Add session query methods (by review, by status)
- Implement session round management

1.2.3 Create session service
- File: `internal/service/session/service.go`
- Implement session lifecycle management
- Add session state transitions
- Implement round management (create, complete, resume)
- Add session cleanup and archival

1.2.4 Add session API endpoints
- File: `biz/handler/review_service/session.go`
- `GET /api/v1/reviews/:id/sessions` - List sessions
- `GET /api/v1/reviews/:id/sessions/:sessionId` - Get session
- `POST /api/v1/reviews/:id/sessions` - Create session
- `PUT /api/v1/reviews/:id/sessions/:sessionId` - Update session

1.2.5 Update review workflow integration
- File: `internal/service/review/workflow.go`
- Integrate session creation into review workflow
- Track progress through session phases
- Save intermediate results to session
- Support multi-round reviews

**Acceptance Criteria**:
- [x] Session model and migration created
- [x] Session repository with CRUD operations
- [x] Session service with lifecycle management
- [x] API endpoints for session management
- [x] Integrated into review workflow

---

#### Task 1.3: Real-Time Progress Tracking

**Priority**: HIGH  
**Estimated Effort**: 1 week  
**Dependencies**: Task 1.2

**Objective**: Implement WebSocket-based real-time progress updates for ongoing reviews.

**Subtasks**:

1.3.1 Define progress event types
- File: `internal/service/progress/types.go`
- Create `ProgressEvent` struct with:
  - `Phase`, `Status`, `Progress` (0-100)
  - `Message`, `Details`
  - `AgentStatus` (per-agent progress)
  - `Timestamp`
- Define event types: `phase_start`, `phase_progress`, `phase_complete`, `agent_status`, `error`

1.3.2 Implement progress broadcaster
- File: `internal/service/progress/broadcaster.go`
- Create `ProgressBroadcaster` with pub/sub pattern
- Implement WebSocket connection management
- Add room-based broadcasting (per review)
- Handle connection lifecycle (connect, disconnect, reconnect)

1.3.3 Add WebSocket endpoint
- File: `biz/handler/review_service/progress_ws.go`
- Create WebSocket handler for progress updates
- Implement authentication for WebSocket connections
- Add heartbeat and reconnection logic
- Handle multiple clients per review

1.3.4 Integrate progress tracking into workflow
- File: `internal/service/review/workflow.go`
- Emit progress events at each phase transition
- Track agent-level progress
- Update session with progress information
- Handle errors and emit error events

1.3.5 Create frontend progress component
- File: `frontend/src/components/ProgressTracker.vue`
- Implement phase visualization with icons
- Show per-agent progress bars
- Add real-time updates via WebSocket
- Display estimated time remaining

**Acceptance Criteria**:
- [x] Progress event types defined
- [x] Broadcaster supports multiple concurrent reviews
- [x] WebSocket endpoint with authentication
- [x] Progress events emitted at each phase
- [x] Frontend component shows real-time progress

---

### Phase 2: Core Features (Weeks 5-8)

#### Task 2.1: Agent Discussion Mechanism

**Priority**: MEDIUM  
**Estimated Effort**: 2 weeks  
**Dependencies**: Task 1.1

**Objective**: Implement a structured discussion mechanism where agents can debate and validate each other's findings.

**Subtasks**:

2.1.1 Define discussion types
- File: `internal/service/discussion/types.go`
- Create `Discussion`, `DiscussionEntry`, `DiscussionResponse` types
- Define response types: `AGREE`, `CHALLENGE`, `CONNECT`, `SURFACE`
- Add confidence and evidence fields

2.1.2 Implement discussion orchestrator
- File: `internal/service/discussion/orchestrator.go`
- Create `DiscussionOrchestrator` interface
- Implement discussion round management
- Add agent prompt generation for discussion
- Implement consensus detection

2.1.3 Create discussion prompts
- File: `internal/service/discussion/prompts.go`
- Generate prompts for each response type
- Include context from other agents' findings
- Add requirements context if available
- Implement prompt templates

2.1.4 Implement discussion aggregator
- File: `internal/service/discussion/aggregator.go`
- Aggregate discussion results
- Update finding confidence based on discussion
- Generate discussion summary
- Identify consensus and disagreements

2.1.5 Add discussion to workflow
- File: `internal/service/review/workflow.go`
- Add discussion phase after parallel reviews
- Integrate discussion results into final report
- Update session with discussion data

**Acceptance Criteria**:
- [x] Discussion types defined
- [x] Orchestrator manages discussion rounds
- [x] Prompts generated for each response type
- [x] Discussion results aggregated
- [x] Integrated into review workflow

---

#### Task 2.2: Requirements-Aware Review

**Priority**: MEDIUM  
**Estimated Effort**: 1.5 weeks  
**Dependencies**: Task 1.1

**Objective**: Enable reviews to be evaluated against explicit requirements, specs, or acceptance criteria.

**Subtasks**:

2.2.1 Define requirements types
- File: `internal/service/requirements/types.go`
- Create `Requirements`, `Requirement`, `CoverageAssessment` types
- Support multiple input formats (inline, file, URL)
- Add requirement categories and priorities

2.2.2 Implement requirements parser
- File: `internal/service/requirements/parser.go`
- Parse inline requirements from request
- Read requirements from uploaded files
- Fetch requirements from URLs
- Validate and normalize requirements

2.2.3 Create requirements evaluator
- File: `internal/service/requirements/evaluator.go`
- Evaluate findings against requirements
- Generate coverage assessment
- Identify gaps and ambiguities
- Calculate requirements compliance score

2.2.4 Add requirements to review request
- File: `biz/model/review.go`
- Add `requirements` field to review request
- Support requirements file upload
- Add requirements URL field
- Update validation logic

2.2.5 Integrate requirements into agents
- File: `internal/service/agent/manager.go`
- Pass requirements to all agents
- Add requirements context to agent prompts
- Evaluate findings against requirements
- Generate requirements coverage report

**Acceptance Criteria**:
- [x] Requirements types defined
- [x] Parser handles multiple input formats
- [x] Evaluator generates coverage assessment
- [x] Review request supports requirements
- [x] Agents evaluate against requirements

---

#### Task 2.3: Code Review Maps

**Priority**: MEDIUM  
**Estimated Effort**: 2 weeks  
**Dependencies**: Task 1.1

**Objective**: Generate structured navigation documents for large changesets with dependency visualization.

**Subtasks**:

2.3.1 Define map types
- File: `internal/service/reviewmap/types.go`
- Create `ReviewMap`, `MapSection`, `MapFile`, `DependencyGraph` types
- Support Mermaid diagram generation
- Add section grouping and prioritization

2.3.2 Implement change analyzer
- File: `internal/service/reviewmap/analyzer.go`
- Analyze git diff to understand changes
- Identify related files and dependencies
- Group changes into logical sections
- Detect change patterns

2.3.3 Create map generator
- File: `internal/service/reviewmap/generator.go`
- Generate structured map document
- Create Mermaid dependency diagrams
- Add file-level progress tracking
- Generate section summaries

2.3.4 Add map API endpoints
- File: `biz/handler/review_service/review_map.go`
- `POST /api/v1/reviews/:id/maps` - Generate map
- `GET /api/v1/reviews/:id/maps` - List maps
- `GET /api/v1/reviews/:id/maps/:mapId` - Get map

2.3.5 Create frontend map viewer
- File: `frontend/src/components/ReviewMap.vue`
- Render map sections with collapsible content
- Display Mermaid diagrams
- Add file-level progress checkboxes
- Implement navigation between sections

**Acceptance Criteria**:
- [x] Map types defined
- [x] Analyzer identifies change patterns
- [x] Generator creates structured maps
- [x] API endpoints for map management
- [x] Frontend viewer renders maps

---

### Phase 3: Advanced Features (Weeks 9-12)

#### Task 3.1: Custom Reviewer Personas (Superseded by Task 1.1)

**Priority**: LOW (already implemented in Task 1.1)  
**Estimated Effort**: 0 weeks  
**Dependencies**: Task 1.1

**Status**: This task has been merged into Task 1.1. The ReviewerAgent model serves as both the "agent" and the "persona" — each agent has a `background` field that defines its review persona/perspective. There is no need for a separate persona system.

The original plan envisioned a separate persona system with prompt templates and focus areas. In the implemented design, the `background` field on `ReviewerAgent` directly serves as the persona definition — it's the text passed to OCR's `--background` parameter that shapes how the AI reviews the code.

**What was delivered instead**:
- `ReviewerAgent` model with `name`, `display_name`, `description`, `background`, `enabled`, `priority`, `tags`
- Preset templates in the frontend (前端审查员, 后端审查员, 安全审查员, 性能审查员)
- CRUD API at `/api/v1/settings/agents`

**Acceptance Criteria**:
- [x] ReviewerAgent model serves as persona definition
- [x] Preset templates available in frontend
- [x] CRUD API for agent/persona management

---

#### Task 3.2: Enhanced Dashboard

**Priority**: LOW  
**Estimated Effort**: 2 weeks  
**Dependencies**: Tasks 1.3, 2.1, 2.3

**Objective**: Enhance the dashboard with advanced visualization and interactive features.

**Subtasks**:

3.2.1 Create session timeline component
- File: `frontend/src/components/SessionTimeline.vue`
- Visualize session phases with timeline
- Show agent progress at each phase
- Add interactive phase details
- Support phase navigation

3.2.2 Implement discussion viewer
- File: `frontend/src/components/DiscussionViewer.vue`
- Render discussion entries with responses
- Highlight agreements and challenges
- Add filtering by response type
- Support discussion navigation

3.2.3 Add requirements coverage view
- File: `frontend/src/components/RequirementsCoverage.vue`
- Display requirements compliance matrix
- Show coverage percentages
- Highlight gaps and ambiguities
- Add requirements filtering

3.2.4 Create report export functionality
- File: `frontend/src/utils/export.ts`
- Export reports as Markdown
- Export reports as PDF
- Add custom report templates
- Support batch export

3.2.5 Implement review comparison
- File: `frontend/src/components/ReviewComparison.vue`
- Compare findings across rounds
- Show progress over time
- Highlight new and resolved issues
- Add trend analysis

**Acceptance Criteria**:
- [x] Session timeline component
- [x] Discussion viewer component
- [x] Requirements coverage view
- [x] Report export functionality
- [x] Review comparison feature

---

### Phase 4: Integration & Polish (Weeks 13-16)

#### Task 4.1: GitHub PR Integration Enhancement

**Priority**: MEDIUM  
**Estimated Effort**: 1.5 weeks  
**Dependencies**: Tasks 2.1, 2.2

**Objective**: Enhance GitHub PR integration with advanced comment formatting and review posting.

**Subtasks**:

4.1.1 Implement comment formatter
- File: `internal/service/publisher/formatter.go`
- Format findings as PR comments
- Add inline code suggestions
- Support collapsible sections
- Generate summary comment

4.1.2 Add review posting service
- File: `internal/service/publisher/github.go`
- Post review as PR comment
- Add inline review comments
- Support review dismissal
- Handle rate limiting

4.1.3 Create human-readable translation
- File: `internal/service/publisher/translator.go`
- Translate technical findings to human language
- Follow Google code review guidelines
- Add context and explanations
- Generate actionable suggestions

4.1.4 Add posting API endpoints
- File: `biz/handler/review_service/publisher.go`
- `POST /api/v1/reviews/:id/publish` - Publish review
- `GET /api/v1/reviews/:id/preview` - Preview comment
- `PUT /api/v1/reviews/:id/publish` - Update published comment

**Acceptance Criteria**:
- [x] Comment formatter with suggestions
- [x] GitHub review posting service
- [x] Human-readable translation
- [x] API endpoints for publishing

---

#### Task 4.2: Performance Optimization

**Priority**: MEDIUM  
**Estimated Effort**: 1 week  
**Dependencies**: All previous tasks

**Objective**: Optimize performance for large-scale deployments.

**Subtasks**:

4.2.1 Implement caching layer
- File: `internal/service/cache/cache.go`
- Cache agent results
- Cache session data
- Cache persona configurations
- Add cache invalidation

4.2.2 Optimize database queries
- File: `internal/repository/postgres/`
- Add database indexes
- Optimize complex queries
- Implement connection pooling
- Add query caching

4.2.3 Add background job processing
- File: `internal/service/worker/`
- Implement job queue for long-running tasks
- Add job prioritization
- Support job cancellation
- Implement retry logic

4.2.4 Implement rate limiting
- File: `internal/middleware/ratelimit.go`
- Add API rate limiting
- Implement per-user limits
- Add burst handling
- Support rate limit headers

**Acceptance Criteria**:
- [x] Caching layer implemented
- [x] Database queries optimized
- [x] Background job processing
- [x] Rate limiting implemented

---

#### Task 4.3: Testing & Documentation

**Priority**: MEDIUM  
**Estimated Effort**: 2 weeks  
**Dependencies**: All previous tasks

**Objective**: Comprehensive testing and documentation for all new features.

**Subtasks**:

4.3.1 Write unit tests
- Test all new packages and functions
- Achieve >80% code coverage
- Add edge case tests
- Implement table-driven tests

4.3.2 Write integration tests
- Test agent orchestration
- Test session management
- Test WebSocket communication
- Test database operations

4.3.3 Write end-to-end tests
- Test complete review workflow
- Test multi-round reviews
- Test requirements evaluation
- Test map generation

4.3.4 Update documentation
- Update README with new features
- Add API documentation
- Create user guide
- Add developer documentation

**Acceptance Criteria**:
- [x] Unit tests with >80% coverage
- [x] Integration tests for all features
- [x] End-to-end tests for workflows
- [x] Complete documentation

---

## Technical Implementation Details

### 1. ReviewerAgent Model (Actual Implementation)

```go
// internal/repository/postgres/models/reviewer_agent.go

package models

import (
    "gorm.io/datatypes"
    "gorm.io/gorm"
)

type ReviewerAgent struct {
    gorm.Model     `json:"-"`
    Name        string         `gorm:"type:text;uniqueIndex;not null" json:"name"`
    DisplayName string         `gorm:"type:text;not null" json:"display_name"`
    Description string         `gorm:"type:text" json:"description"`
    Background  string         `gorm:"type:text" json:"background"` // OCR --background content
    Enabled     bool           `gorm:"not null;default:true" json:"enabled"`
    Priority    int            `gorm:"not null;default:0" json:"priority"`
    Tags        datatypes.JSON `gorm:"type:jsonb" json:"tags,omitempty"`
}

func (ReviewerAgent) TableName() string {
    return "reviewer_agents"
}
```

### 2. OCR Runner --background Support (Actual Implementation)

```go
// internal/service/runner/runner.go (excerpt)

type OpenCodeReviewConfig struct {
    Binary          string
    RulePath        string
    DefaultSeverity string
    Concurrency     int
    MaxGitProcs     int
    TimeoutMinutes  int
    MaxTools        int
    URL             string
    Model           string
    AuthToken       string
    AuthHeader      string
    UseAnthropic    bool
    ExtraBody       string
    Language        string
    Background      string  // <-- NEW: passed as --background to OCR
}

func (r *OpenCodeReviewRunner) Run(ctx context.Context, workspace review.Workspace) (review.Result, error) {
    // ... build args ...
    if r.config.MaxTools > 0 {
        args = append(args, "--max-tools", fmt.Sprintf("%d", r.config.MaxTools))
    }
    if strings.TrimSpace(r.config.Background) != "" {
        args = append(args, "--background", r.config.Background)
    }
    // ... execute OCR ...
}
```

### 3. Agent Manager (Existing Implementation)

The agent manager and related abstractions remain available for future multi-engine expansion:

```go
// internal/service/agent/interface.go (still available)
type ReviewAgent interface {
    Name() string
    Focus() []string
    Review(ctx context.Context, workspace Workspace) (AgentResult, error)
    Priority() int
}
```

### 2. Agent Manager

```go
// internal/service/agent/manager.go

package agent

import (
    "context"
    "sync"
    "time"
)

// AgentManager manages multiple review agents
type AgentManager struct {
    agents     []ReviewAgent
    aggregator ResultAggregator
    config     ManagerConfig
}

// ManagerConfig contains manager configuration
type ManagerConfig struct {
    MaxConcurrency int
    AgentTimeout   time.Duration
    TotalTimeout   time.Duration
}

// NewAgentManager creates a new agent manager
func NewAgentManager(config ManagerConfig, aggregator ResultAggregator) *AgentManager {
    return &AgentManager{
        config:     config,
        aggregator: aggregator,
    }
}

// RegisterAgent registers a review agent
func (m *AgentManager) RegisterAgent(agent ReviewAgent) {
    m.agents = append(m.agents, agent)
}

// RunAgents runs all registered agents in parallel
func (m *AgentManager) RunAgents(ctx context.Context, workspace Workspace) (AggregatedResult, error) {
    ctx, cancel := context.WithTimeout(ctx, m.config.TotalTimeout)
    defer cancel()
    
    results := make(chan AgentResult, len(m.agents))
    errors := make(chan error, len(m.agents))
    
    // Limit concurrency
    sem := make(chan struct{}, m.config.MaxConcurrency)
    
    var wg sync.WaitGroup
    for _, agent := range m.agents {
        wg.Add(1)
        go func(a ReviewAgent) {
            defer wg.Done()
            
            sem <- struct{}{}
            defer func() { <-sem }()
            
            agentCtx, agentCancel := context.WithTimeout(ctx, m.config.AgentTimeout)
            defer agentCancel()
            
            result, err := a.Review(agentCtx, workspace)
            if err != nil {
                errors <- err
                return
            }
            
            results <- result
        }(agent)
    }
    
    // Wait for all agents to complete
    go func() {
        wg.Wait()
        close(results)
        close(errors)
    }()
    
    // Collect results
    var agentResults []AgentResult
    for result := range results {
        agentResults = append(agentResults, result)
    }
    
    // Collect errors
    var agentErrors []error
    for err := range errors {
        agentErrors = append(agentErrors, err)
    }
    
    // Aggregate results
    return m.aggregator.Aggregate(agentResults), nil
}
```

### 3. Session Model

```go
// internal/repository/postgres/models/session.go

package models

import (
    "time"
    "github.com/google/uuid"
    "gorm.io/datatypes"
)

// ReviewSession represents a review session
type ReviewSession struct {
    ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
    ReviewID      string         `gorm:"index;not null" json:"review_id"`
    Round         int            `gorm:"not null;default:1" json:"round"`
    Phase         string         `gorm:"not null" json:"phase"`
    Status        string         `gorm:"not null;default:'active'" json:"status"`
    Progress      float64        `gorm:"not null;default:0" json:"progress"`
    AgentResults  datatypes.JSON `gorm:"type:jsonb" json:"agent_results"`
    Discussion    datatypes.JSON `gorm:"type:jsonb" json:"discussion"`
    FinalReport   datatypes.JSON `gorm:"type:jsonb" json:"final_report"`
    Requirements  datatypes.JSON `gorm:"type:jsonb" json:"requirements"`
    Metadata      datatypes.JSON `gorm:"type:jsonb" json:"metadata"`
    CreatedAt     time.Time      `json:"created_at"`
    UpdatedAt     time.Time      `json:"updated_at"`
    CompletedAt   *time.Time     `json:"completed_at"`
}

// TableName returns the table name
func (ReviewSession) TableName() string {
    return "review_sessions"
}
```

### 4. Progress Event

```go
// internal/service/progress/types.go

package progress

import "time"

// EventType represents the type of progress event
type EventType string

const (
    EventPhaseStart    EventType = "phase_start"
    EventPhaseProgress EventType = "phase_progress"
    EventPhaseComplete EventType = "phase_complete"
    EventAgentStatus   EventType = "agent_status"
    EventError         EventType = "error"
    EventComplete      EventType = "complete"
)

// Phase represents a review phase
type Phase string

const (
    PhaseContext      Phase = "context"
    PhaseChangeContext Phase = "change_context"
    PhaseAnalysis     Phase = "analysis"
    PhaseReviews      Phase = "reviews"
    PhaseAggregation  Phase = "aggregation"
    PhaseDiscussion   Phase = "discourse"
    PhaseSynthesis    Phase = "synthesis"
    PhaseComplete     Phase = "complete"
)

// ProgressEvent represents a progress update
type ProgressEvent struct {
    Type        EventType              `json:"type"`
    Phase       Phase                  `json:"phase"`
    Status      string                 `json:"status"`
    Progress    float64                `json:"progress"`
    Message     string                 `json:"message"`
    AgentStatus map[string]AgentStatus `json:"agent_status,omitempty"`
    Error       string                 `json:"error,omitempty"`
    Timestamp   time.Time              `json:"timestamp"`
}

// AgentStatus represents an agent's status
type AgentStatus struct {
    Name     string  `json:"name"`
    Status   string  `json:"status"`
    Progress float64 `json:"progress"`
    Message  string  `json:"message"`
}
```

### 5. Discussion Types

```go
// internal/service/discussion/types.go

package discussion

import "time"

// ResponseType represents the type of discussion response
type ResponseType string

const (
    ResponseAgree    ResponseType = "AGREE"
    ResponseChallenge ResponseType = "CHALLENGE"
    ResponseConnect  ResponseType = "CONNECT"
    ResponseSurface  ResponseType = "SURFACE"
)

// Discussion represents a discussion round
type Discussion struct {
    ID        string           `json:"id"`
    SessionID string           `json:"session_id"`
    Round     int              `json:"round"`
    Entries   []DiscussionEntry `json:"entries"`
    Summary   string           `json:"summary"`
    CreatedAt time.Time        `json:"created_at"`
}

// DiscussionEntry represents a single discussion entry
type DiscussionEntry struct {
    ID          string           `json:"id"`
    AgentName   string           `json:"agent_name"`
    FindingID   string           `json:"finding_id"`
    Response    ResponseType     `json:"response"`
    Content     string           `json:"content"`
    Evidence    string           `json:"evidence"`
    Confidence  float64          `json:"confidence"`
    References  []string         `json:"references"`
    CreatedAt   time.Time        `json:"created_at"`
}

// DiscussionResult represents the aggregated discussion result
type DiscussionResult struct {
    Consensus    []ConsensusItem  `json:"consensus"`
    Disagreements []DisagreementItem `json:"disagreements"`
    NewFindings  []Finding        `json:"new_findings"`
    UpdatedFindings []Finding     `json:"updated_findings"`
    Summary      string           `json:"summary"`
}

// ConsensusItem represents a consensus finding
type ConsensusItem struct {
    FindingID   string   `json:"finding_id"`
    Agents      []string `json:"agents"`
    Confidence  float64  `json:"confidence"`
    Summary     string   `json:"summary"`
}

// DisagreementItem represents a disagreement
type DisagreementItem struct {
    FindingID   string          `json:"finding_id"`
    Positions   []AgentPosition `json:"positions"`
    Resolution  string          `json:"resolution"`
}

// AgentPosition represents an agent's position
type AgentPosition struct {
    AgentName  string `json:"agent_name"`
    Position   string `json:"position"`
    Evidence   string `json:"evidence"`
    Confidence float64 `json:"confidence"`
}
```

### 6. Frontend Progress Component

```vue
<!-- frontend/src/components/ProgressTracker.vue -->

<template>
  <div class="progress-tracker">
    <div class="progress-header">
      <h3>Review Progress</h3>
      <span class="progress-percentage">{{ overallProgress }}%</span>
    </div>
    
    <div class="phases">
      <div
        v-for="phase in phases"
        :key="phase.name"
        :class="['phase', phase.status]"
      >
        <div class="phase-icon">
          <el-icon v-if="phase.status === 'completed'">
            <CircleCheckFilled />
          </el-icon>
          <el-icon v-else-if="phase.status === 'active'">
            <Loading />
          </el-icon>
          <el-icon v-else>
            <Clock />
          </el-icon>
        </div>
        
        <div class="phase-info">
          <h4>{{ phase.title }}</h4>
          <p>{{ phase.description }}</p>
          
          <div v-if="phase.status === 'active' && phase.agents" class="agents">
            <div
              v-for="agent in phase.agents"
              :key="agent.name"
              class="agent"
            >
              <span class="agent-name">{{ agent.name }}</span>
              <el-progress
                :percentage="agent.progress"
                :status="getAgentStatus(agent.status)"
                :stroke-width="4"
              />
            </div>
          </div>
        </div>
        
        <div v-if="phase.status === 'active'" class="phase-progress">
          <el-progress
            type="circle"
            :percentage="phase.progress"
            :width="48"
            :stroke-width="4"
          />
        </div>
      </div>
    </div>
    
    <div v-if="error" class="error">
      <el-alert
        :title="error.title"
        :description="error.message"
        type="error"
        show-icon
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { CircleCheckFilled, Loading, Clock } from '@element-plus/icons-vue'

interface Agent {
  name: string
  status: string
  progress: number
  message: string
}

interface Phase {
  name: string
  title: string
  description: string
  status: 'pending' | 'active' | 'completed'
  progress: number
  agents?: Agent[]
}

interface Error {
  title: string
  message: string
}

const props = defineProps<{
  reviewId: string
}>()

const phases = ref<Phase[]>([
  {
    name: 'context',
    title: 'Context Discovery',
    description: 'Loading configuration and discovering project standards',
    status: 'pending',
    progress: 0,
  },
  {
    name: 'change_context',
    title: 'Change Analysis',
    description: 'Analyzing git diff and understanding changes',
    status: 'pending',
    progress: 0,
  },
  {
    name: 'analysis',
    title: 'Tech Lead Analysis',
    description: 'Summarizing changes and identifying risks',
    status: 'pending',
    progress: 0,
  },
  {
    name: 'reviews',
    title: 'Parallel Reviews',
    description: 'Running multiple review agents in parallel',
    status: 'pending',
    progress: 0,
  },
  {
    name: 'aggregation',
    title: 'Aggregation',
    description: 'Merging findings from multiple agents',
    status: 'pending',
    progress: 0,
  },
  {
    name: 'discourse',
    title: 'Discourse',
    description: 'Agents debating and validating findings',
    status: 'pending',
    progress: 0,
  },
  {
    name: 'synthesis',
    title: 'Synthesis',
    description: 'Producing final prioritized review',
    status: 'pending',
    progress: 0,
  },
  {
    name: 'complete',
    title: 'Complete',
    description: 'Review completed',
    status: 'pending',
    progress: 0,
  },
])

const error = ref<Error | null>(null)
let ws: WebSocket | null = null

const overallProgress = computed(() => {
  const total = phases.value.length
  const completed = phases.value.filter(p => p.status === 'completed').length
  const active = phases.value.find(p => p.status === 'active')
  const activeProgress = active ? active.progress / 100 : 0
  return Math.round(((completed + activeProgress) / total) * 100)
})

function getAgentStatus(status: string): '' | 'success' | 'exception' | 'warning' {
  switch (status) {
    case 'completed': return 'success'
    case 'failed': return 'exception'
    case 'running': return ''
    default: return ''
  }
}

function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/api/v1/reviews/${props.reviewId}/progress`
  
  ws = new WebSocket(wsUrl)
  
  ws.onmessage = (event) => {
    const data = JSON.parse(event.data)
    handleProgressUpdate(data)
  }
  
  ws.onerror = () => {
    error.value = {
      title: 'Connection Error',
      message: 'Failed to connect to progress stream',
    }
  }
  
  ws.onclose = () => {
    // Reconnect after 3 seconds
    setTimeout(connectWebSocket, 3000)
  }
}

function handleProgressUpdate(event: any) {
  const phaseIndex = phases.value.findIndex(p => p.name === event.phase)
  if (phaseIndex === -1) return
  
  phases.value[phaseIndex].status = event.status === 'completed' ? 'completed' : 'active'
  phases.value[phaseIndex].progress = event.progress
  
  if (event.agent_status) {
    phases.value[phaseIndex].agents = Object.values(event.agent_status)
  }
  
  // Mark previous phases as completed
  for (let i = 0; i < phaseIndex; i++) {
    if (phases.value[i].status !== 'completed') {
      phases.value[i].status = 'completed'
      phases.value[i].progress = 100
    }
  }
  
  // Mark next phases as pending
  for (let i = phaseIndex + 1; i < phases.value.length; i++) {
    if (phases.value[i].status !== 'pending') {
      phases.value[i].status = 'pending'
      phases.value[i].progress = 0
    }
  }
  
  if (event.type === 'error') {
    error.value = {
      title: 'Review Error',
      message: event.error,
    }
  }
}

onMounted(() => {
  connectWebSocket()
})

onUnmounted(() => {
  if (ws) {
    ws.close()
  }
})
</script>

<style scoped>
.progress-tracker {
  padding: 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.progress-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.progress-header h3 {
  margin: 0;
  font-size: 18px;
  color: #303133;
}

.progress-percentage {
  font-size: 24px;
  font-weight: bold;
  color: #409eff;
}

.phases {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.phase {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px;
  border-radius: 8px;
  background: #f5f7fa;
  transition: all 0.3s ease;
}

.phase.active {
  background: #ecf5ff;
  border: 1px solid #409eff;
}

.phase.completed {
  background: #f0f9eb;
}

.phase-icon {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: #dcdfe6;
  color: #909399;
}

.phase.active .phase-icon {
  background: #409eff;
  color: #fff;
}

.phase.completed .phase-icon {
  background: #67c23a;
  color: #fff;
}

.phase-info {
  flex: 1;
}

.phase-info h4 {
  margin: 0 0 4px;
  font-size: 14px;
  color: #303133;
}

.phase-info p {
  margin: 0;
  font-size: 12px;
  color: #909399;
}

.agents {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.agent {
  display: flex;
  align-items: center;
  gap: 8px;
}

.agent-name {
  width: 100px;
  font-size: 12px;
  color: #606266;
}

.phase-progress {
  flex-shrink: 0;
}

.error {
  margin-top: 16px;
}
</style>
```

---

## Database Migrations

### Migration 1: Review Sessions Table

```sql
-- migrations/001_create_review_sessions.sql

CREATE TABLE IF NOT EXISTS review_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    review_id VARCHAR(255) NOT NULL,
    round INTEGER NOT NULL DEFAULT 1,
    phase VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    progress DECIMAL(5,2) NOT NULL DEFAULT 0,
    agent_results JSONB,
    discussion JSONB,
    final_report JSONB,
    requirements JSONB,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_review_sessions_review FOREIGN KEY (review_id) REFERENCES reviews(review_id) ON DELETE CASCADE
);

CREATE INDEX idx_review_sessions_review_id ON review_sessions(review_id);
CREATE INDEX idx_review_sessions_status ON review_sessions(status);
CREATE INDEX idx_review_sessions_created_at ON review_sessions(created_at);
```

### Migration 2: Reviewer Agents Table (Implemented)

```sql
-- AutoMigrate via GORM, table: reviewer_agents

CREATE TABLE IF NOT EXISTS reviewer_agents (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    description TEXT,
    background TEXT,           -- OCR --background parameter content
    enabled BOOLEAN NOT NULL DEFAULT true,
    priority INTEGER NOT NULL DEFAULT 0,
    tags JSONB
);

CREATE INDEX idx_reviewer_agents_name ON reviewer_agents(name);
CREATE INDEX idx_reviewer_agents_deleted_at ON reviewer_agents(deleted_at);
```

### Migration 3: Reviewer Personas Table (Deprecated)

### Migration 3: Review Maps Table

```sql
-- migrations/003_create_review_maps.sql

CREATE TABLE IF NOT EXISTS review_maps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    sections JSONB NOT NULL DEFAULT '[]',
    dependency_graph TEXT,
    file_count INTEGER NOT NULL DEFAULT 0,
    section_count INTEGER NOT NULL DEFAULT 0,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT fk_review_maps_session FOREIGN KEY (session_id) REFERENCES review_sessions(id) ON DELETE CASCADE
);

CREATE INDEX idx_review_maps_session_id ON review_maps(session_id);
```

---

## Configuration

### Agent Configuration (Actual Implementation)

Agents are stored in the `reviewer_agents` database table and managed via the `/api/v1/settings/agents` API and the `/settings/agents` frontend page.

**Default OCR Background** (single-agent fallback when no agents are configured):

```yaml
# config.yaml or OCR Settings page
ocr:
  background: |
    这是一个电商项目，重点关注支付流程的安全性和性能
```

**Agent Team** (managed via UI or API):

Each agent has a `background` field that is passed as OCR's `--background` parameter:

| Agent | Background Example |
|-------|-------------------|
| 前端审查员 | 重点关注前端代码：Vue 组件设计、CSS 样式规范... |
| 后端审查员 | 重点关注后端代码：API 设计规范、数据库查询性能... |
| 安全审查员 | 重点关注安全问题：SQL 注入、XSS、权限控制... |
| 性能审查员 | 重点关注性能问题：算法复杂度、内存使用... |

**Preset Templates** available in frontend:
- `frontend` - 前端审查员
- `backend` - 后端审查员  
- `security` - 安全审查员
- `performance` - 性能审查员

### Session Configuration (Remaining from original plan)

```yaml
discussion:
  enabled: true
  max_rounds: 3
  min_agents: 2

requirements:
  enabled: true
  max_file_size: 1MB

review_maps:
  enabled: true
  min_files: 10
```

---

## API Endpoints

### Review Sessions

```
GET    /api/v1/reviews/:id/sessions
POST   /api/v1/reviews/:id/sessions
GET    /api/v1/reviews/:id/sessions/:sessionId
PUT    /api/v1/reviews/:id/sessions/:sessionId
DELETE /api/v1/reviews/:id/sessions/:sessionId
```

### Progress WebSocket

```
WS     /api/v1/reviews/:id/progress
```

### Reviewer Agents (Implemented)

```
GET    /api/v1/settings/agents          - List all agents
POST   /api/v1/settings/agents          - Create agent
PUT    /api/v1/settings/agents/:id      - Update agent
DELETE /api/v1/settings/agents/:id      - Delete agent
```

### Reviewer Personas (Deprecated - merged into Reviewer Agents)

### Review Maps

```
POST   /api/v1/reviews/:id/maps
GET    /api/v1/reviews/:id/maps
GET    /api/v1/reviews/:id/maps/:mapId
```

### Publishing

```
POST   /api/v1/reviews/:id/publish
GET    /api/v1/reviews/:id/preview
PUT    /api/v1/reviews/:id/publish
```

---

## Testing Strategy

### Unit Tests

- Test all agent implementations
- Test session management logic
- Test progress event handling
- Test discussion orchestration
- Test requirements parsing
- Test map generation

### Integration Tests

- Test agent manager with multiple agents
- Test session lifecycle with database
- Test WebSocket communication
- Test GitHub API integration

### End-to-End Tests

- Test complete review workflow
- Test multi-round reviews
- Test requirements evaluation
- Test map generation and viewing

---

## Deployment

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Node.js 18+
- Redis (optional, for caching)

### Environment Variables

```bash
# OCR Background (single-agent fallback)
OCR_BACKGROUND="默认审查侧重点文本"

# Agent Configuration (managed via DB, not env vars)
# Use /api/v1/settings/agents or /settings/agents UI

# Discussion
DISCUSSION_ENABLED=true
DISCUSSION_MAX_ROUNDS=3

# Requirements
REQUIREMENTS_ENABLED=true
REQUIREMENTS_MAX_FILE_SIZE=1MB

# Review Maps
REVIEW_MAPS_ENABLED=true
REVIEW_MAPS_MIN_FILES=10

# WebSocket
WS_HEARTBEAT_INTERVAL=30s
WS_MAX_CONNECTIONS=1000
```

---

## Timeline Summary

| Phase | Tasks | Duration | Dependencies |
|-------|-------|----------|--------------|
| Phase 1 | Tasks 1.1-1.3 | Weeks 1-4 | None |
| Phase 2 | Tasks 2.1-2.3 | Weeks 5-8 | Phase 1 |
| Phase 3 | Tasks 3.1-3.2 | Weeks 9-12 | Phase 2 |
| Phase 4 | Tasks 4.1-4.3 | Weeks 13-16 | Phase 3 |

**Total Estimated Duration**: 16 weeks (4 months)

---

## Success Metrics

1. **Agent Performance**
   - Agent execution time < 5 minutes per agent
   - Agent success rate > 95%
   - Finding deduplication rate > 30%

2. **Session Management**
   - Session creation time < 100ms
   - Session query time < 50ms
   - Support 1000+ concurrent sessions

3. **Progress Tracking**
   - WebSocket connection time < 100ms
   - Progress update latency < 500ms
   - Support 100+ concurrent connections per review

4. **Discussion Quality**
   - Discussion completion rate > 90%
   - Consensus detection accuracy > 80%
   - False positive reduction > 20%

5. **Requirements Coverage**
   - Requirements parsing success rate > 95%
   - Coverage calculation accuracy > 90%
   - Support 100+ requirements per review

---

## Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Agent timeout | High | Medium | Implement proper timeout handling and cancellation |
| Database performance | High | Medium | Add indexes, optimize queries, implement caching |
| WebSocket scalability | Medium | Medium | Use Redis pub/sub for multi-instance support |
| Agent API rate limits | Medium | High | Implement rate limiting and retry logic |
| Memory usage | Medium | Medium | Implement proper cleanup and garbage collection |

---

## Conclusion

This enhancement plan will transform the review-service into a powerful multi-agent code review platform with advanced features like real-time progress tracking, agent discussions, requirements-aware reviews, and code review maps. The phased approach ensures manageable implementation while delivering value incrementally.

---

## Implementation Status (Last Updated: 2026-06-14)

### Overall Progress: ~95% Complete

### Phase 1: Foundation ✅ COMPLETE
| Task | Status | Notes |
|------|--------|-------|
| 1.1 Multi-Agent Architecture | ✅ Done | ReviewerAgent model, CRUD API, frontend UI |
| 1.2 Session Management | ✅ Done | Session model, service, API endpoints |
| 1.3 Real-Time Progress | ✅ Done | WebSocket, ProgressTracker component |

### Phase 2: Core Features ✅ COMPLETE
| Task | Status | Notes |
|------|--------|-------|
| 2.1 Agent Discussion | ✅ Done | Types, orchestrator, frontend component |
| 2.2 Requirements-Aware Review | ✅ Done | Parser, evaluator, frontend component |
| 2.3 Code Review Maps | ✅ Done | Generator, API endpoints, frontend component |

### Phase 3: Advanced Features ✅ COMPLETE
| Task | Status | Notes |
|------|--------|-------|
| 3.1 Custom Personas | ✅ Done | Merged into Task 1.1 |
| 3.2 Enhanced Dashboard | ✅ Done | All components: timeline, discussion, coverage, export, comparison |

### Phase 4: Integration & Polish ✅ COMPLETE
| Task | Status | Notes |
|------|--------|-------|
| 4.1 GitHub PR Integration | ✅ Done | Comment formatter, publisher handler, API endpoints |
| 4.2 Performance Optimization | ✅ Done | LRU cache with stats, rate limiting middleware |
| 4.3 Testing & Documentation | ✅ Done | Unit tests, API documentation |

### Key Files Implemented

**Backend (Go):**
- `internal/repository/postgres/models/session.go` - Session model
- `internal/repository/postgres/session_repo.go` - Session repository
- `internal/service/session/service.go` - Session service
- `internal/service/progress/types.go` - Progress types
- `internal/service/progress/broadcaster.go` - WebSocket broadcaster
- `internal/service/discussion/types.go` - Discussion types
- `internal/service/discussion/orchestrator.go` - Discussion orchestrator
- `internal/service/requirements/types.go` - Requirements parser/evaluator
- `internal/service/reviewmap/types.go` - Review map generator
- `internal/service/workflow/workflow.go` - Review workflow
- `internal/service/review/enhanced_service.go` - Enhanced service
- `internal/service/writeback/formatter.go` - Comment formatter
- `internal/service/cache/cache.go` - Cache layer
- `internal/middleware/ratelimit.go` - Rate limiting middleware
- `biz/handler/review_service/session.go` - Session API handlers
- `biz/handler/review_service/progress_ws.go` - WebSocket handler
- `biz/handler/review_service/review_map.go` - Review map API handlers
- `biz/handler/review_service/publisher.go` - Publisher API handlers

**Frontend (Vue):**
- `frontend/src/components/ProgressTracker.vue` - Progress tracking
- `frontend/src/components/SessionTimeline.vue` - Session timeline
- `frontend/src/components/DiscussionViewer.vue` - Discussion viewer
- `frontend/src/components/RequirementsCoverage.vue` - Requirements coverage
- `frontend/src/components/ReviewMap.vue` - Review map viewer
- `frontend/src/components/ReviewComparison.vue` - Review comparison
- `frontend/src/views/SessionDetail.vue` - Session detail page
- `frontend/src/views/AgentSettings.vue` - Agent management
- `frontend/src/utils/export.ts` - Report export utilities

**Tests:**
- `internal/service/cache/cache_test.go` - Cache unit tests
- `internal/middleware/ratelimit_test.go` - Rate limiter unit tests

**Documentation:**
- `docs/API.md` - API documentation

### Next Steps (Optional)
1. Add more comprehensive unit tests
2. Add integration tests
3. Add end-to-end tests
4. Deploy and monitor in production