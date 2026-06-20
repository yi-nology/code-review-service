package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/oekit/gormx"
)

// Ensure MockSessionService implements sessionService interface
var _ sessionService = (*MockSessionService)(nil)

// MockSessionService is a mock implementation of session.Service
type MockSessionService struct {
	sessions map[string]*models.ReviewSession
}

func NewMockSessionService() *MockSessionService {
	return &MockSessionService{
		sessions: make(map[string]*models.ReviewSession),
	}
}

func (m *MockSessionService) CreateSession(ctx context.Context, reviewID string, round int) (*models.ReviewSession, error) {
	sessionID := "test-session-id"
	session := &models.ReviewSession{
		SessionID: sessionID,
		ReviewID:  reviewID,
		Round:     round,
		Phase:     "context",
		Status:    "active",
		Progress:  0,
	}
	m.sessions[sessionID] = session
	return session, nil
}

func (m *MockSessionService) GetSession(ctx context.Context, sessionID string) (*models.ReviewSession, error) {
	if session, ok := m.sessions[sessionID]; ok {
		return session, nil
	}
	return nil, nil
}

func (m *MockSessionService) GetActiveSession(ctx context.Context, reviewID string) (*models.ReviewSession, error) {
	for _, session := range m.sessions {
		if session.ReviewID == reviewID && session.Status == "active" {
			return session, nil
		}
	}
	return nil, nil
}

func (m *MockSessionService) GetSessionsByReview(ctx context.Context, reviewID string) ([]*models.ReviewSession, error) {
	var sessions []*models.ReviewSession
	for _, session := range m.sessions {
		if session.ReviewID == reviewID {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

func (m *MockSessionService) UpdatePhase(ctx context.Context, sessionID string, phase models.SessionPhase, progress float64) error {
	if session, ok := m.sessions[sessionID]; ok {
		session.Phase = string(phase)
		session.Progress = progress
	}
	return nil
}

func (m *MockSessionService) UpdateStatus(ctx context.Context, sessionID string, status models.SessionStatus) error {
	if session, ok := m.sessions[sessionID]; ok {
		session.Status = string(status)
	}
	return nil
}

func (m *MockSessionService) UpdateAgentResults(ctx context.Context, sessionID string, agentResults []models.AgentResultData) error {
	return nil
}

func (m *MockSessionService) UpdateDiscussion(ctx context.Context, sessionID string, discussion models.DiscussionData) error {
	return nil
}

func (m *MockSessionService) UpdateFinalReport(ctx context.Context, sessionID string, finalReport models.FinalReportData) error {
	return nil
}

func (m *MockSessionService) CompleteSession(ctx context.Context, sessionID string, finalReport models.FinalReportData) error {
	if session, ok := m.sessions[sessionID]; ok {
		session.Status = "completed"
		now := time.Now()
		session.CompletedAt = &now
	}
	return nil
}

func (m *MockSessionService) FailSession(ctx context.Context, sessionID string, reason string) error {
	if session, ok := m.sessions[sessionID]; ok {
		session.Status = "failed"
	}
	return nil
}

func (m *MockSessionService) CancelSession(ctx context.Context, sessionID string) error {
	if session, ok := m.sessions[sessionID]; ok {
		session.Status = "cancelled"
	}
	return nil
}

func (m *MockSessionService) GetNextRound(ctx context.Context, reviewID string) (int, error) {
	return 1, nil
}

func (m *MockSessionService) ListSessions(ctx context.Context, page, pageSize int) (*gormx.PageResponse[models.ReviewSession], error) {
	var sessions []models.ReviewSession
	for _, session := range m.sessions {
		sessions = append(sessions, *session)
	}
	return &gormx.PageResponse[models.ReviewSession]{
		List:  sessions,
		Total: int64(len(sessions)),
		Page:  page,
		Pages: 1,
	}, nil
}

// MockAgent is a mock implementation of agent.ReviewAgent
type MockAgent struct {
	name     string
	focus    []string
	priority int
}

func NewMockAgent(name string, focus []string, priority int) *MockAgent {
	return &MockAgent{
		name:     name,
		focus:    focus,
		priority: priority,
	}
}

func (a *MockAgent) Name() string {
	return a.name
}

func (a *MockAgent) Focus() []string {
	return a.focus
}

func (a *MockAgent) Priority() int {
	return a.priority
}

func (a *MockAgent) Review(ctx context.Context, workspace agent.Workspace) (agent.AgentResult, error) {
	return agent.AgentResult{
		AgentName: a.name,
		Findings: []agent.Finding{
			{
				ID:       "finding-1",
				Severity: "major",
				Category: "code_quality",
				Path:     "test.go",
				Line:     10,
				Title:    "Test finding",
				Body:     "This is a test finding",
				Agents:   []string{a.name},
			},
		},
		Summary:    "Test review completed",
		Confidence: 0.8,
		Duration:   time.Second,
	}, nil
}

func TestReviewWorkflow(t *testing.T) {
	// Create mock services
	sessionService := NewMockSessionService()
	progressManager := progress.NewProgressManager()

	// Create agent manager
	aggregator := agent.NewDefaultAggregator(agent.DefaultAggregatorConfig())
	agentManager := agent.NewAgentManager(agent.DefaultManagerConfig(), aggregator)

	// Register mock agents
	agentManager.RegisterAgent(NewMockAgent("principal", []string{"architecture"}, 1))
	agentManager.RegisterAgent(NewMockAgent("quality", []string{"code_style"}, 2))

	// Create workflow
	workflow := NewReviewWorkflow(
		agentManager,
		sessionService,
		progressManager,
		DefaultWorkflowConfig(),
	)

	// Create review request
	request := ReviewRequest{
		ReviewID: "test-review-id",
		Provider: "github",
		Repo:     "test/repo",
		BaseSHA:  "abc123",
		HeadSHA:  "def456",
		Diff:     "test diff",
		Dir:      "/tmp/test",
		Round:    1,
	}

	// Run review
	ctx := context.Background()
	result, err := workflow.RunReview(ctx, request)
	if err != nil {
		t.Fatalf("RunReview failed: %v", err)
	}

	// Verify result
	if result == nil {
		t.Fatal("result is nil")
	}

	if result.SessionID == "" {
		t.Error("session ID is empty")
	}

	if result.Round != 1 {
		t.Errorf("expected round 1, got %d", result.Round)
	}

	if len(result.AgentResults) != 2 {
		t.Errorf("expected 2 agent results, got %d", len(result.AgentResults))
	}

	if result.FinalReport.Score < 0 || result.FinalReport.Score > 100 {
		t.Errorf("invalid score: %d", result.FinalReport.Score)
	}

	t.Logf("Review completed with score: %d", result.FinalReport.Score)
	t.Logf("Verdict: %s", result.FinalReport.Verdict)
	t.Logf("Findings: %d", len(result.AgentResults[0].Findings)+len(result.AgentResults[1].Findings))
}
