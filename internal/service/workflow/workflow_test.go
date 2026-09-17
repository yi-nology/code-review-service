package workflow

import (
	"context"
	"testing"
	"time"

	dsession "github.com/yi-nology/code-review-service/internal/domain/session"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
)

// Ensure MockSessionService implements sessionService interface
var _ sessionService = (*MockSessionService)(nil)

// MockSessionService is a mock implementation of session.Service
type MockSessionService struct {
	sessions map[string]*dsession.Session
}

func NewMockSessionService() *MockSessionService {
	return &MockSessionService{
		sessions: make(map[string]*dsession.Session),
	}
}

func (m *MockSessionService) CreateSession(ctx context.Context, reviewID string, round int) (*dsession.Session, error) {
	sessionID := "test-session-id"
	session := &dsession.Session{
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

func (m *MockSessionService) GetSession(ctx context.Context, sessionID string) (*dsession.Session, error) {
	if session, ok := m.sessions[sessionID]; ok {
		return session, nil
	}
	return nil, nil
}

func (m *MockSessionService) GetActiveSession(ctx context.Context, reviewID string) (*dsession.Session, error) {
	for _, session := range m.sessions {
		if session.ReviewID == reviewID && session.Status == "active" {
			return session, nil
		}
	}
	return nil, nil
}

func (m *MockSessionService) GetSessionsByReview(ctx context.Context, reviewID string) ([]*dsession.Session, error) {
	var sessions []*dsession.Session
	for _, session := range m.sessions {
		if session.ReviewID == reviewID {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

func (m *MockSessionService) UpdatePhase(ctx context.Context, sessionID string, phase string, progress float64) error {
	if session, ok := m.sessions[sessionID]; ok {
		session.Phase = string(phase)
		session.Progress = progress
	}
	return nil
}

func (m *MockSessionService) UpdateStatus(ctx context.Context, sessionID string, status string) error {
	if session, ok := m.sessions[sessionID]; ok {
		session.Status = string(status)
	}
	return nil
}

func (m *MockSessionService) UpdateAgentResults(ctx context.Context, sessionID string, agentResults []dsession.AgentResultData) error {
	return nil
}

func (m *MockSessionService) UpdateDiscussion(ctx context.Context, sessionID string, discussion dsession.DiscussionData) error {
	return nil
}

func (m *MockSessionService) UpdateFinalReport(ctx context.Context, sessionID string, finalReport dsession.FinalReportData) error {
	return nil
}

func (m *MockSessionService) CompleteSession(ctx context.Context, sessionID string, finalReport dsession.FinalReportData) error {
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

func (m *MockSessionService) ListSessions(ctx context.Context, page, pageSize int) (*gormx.PageResponse[dsession.Session], error) {
	var sessions []dsession.Session
	for _, session := range m.sessions {
		sessions = append(sessions, *session)
	}
	return &gormx.PageResponse[dsession.Session]{
		Items: sessions,
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

func (a *MockAgent) Persona() string {
	return a.name
}

func (a *MockAgent) Focus() []string {
	return a.focus
}

func (a *MockAgent) Priority() int {
	return a.priority
}

func (a *MockAgent) IsDefault() bool {
	return false
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

	// Create workflow (NoopProvider → discourse 走降级路径)
	workflow := NewReviewWorkflow(
		agentManager,
		sessionService,
		progressManager,
		ocr.NewNoopProvider(),
		WorkflowConfig{
			EnableDiscussion:   true,
			EnableRequirements: true,
			DiscussionRounds:   1,
		},
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

// TestGenerateFinalReportSeverityCounts 验证 generateFinalReport 按「真实 severity」计数，
// 而非按 bucket 粗分。这是保证 workflow 路径分数与 executor 的 BuildReport 一致的前提：
// info 问题不能被误计为 minor。
func TestGenerateFinalReportSeverityCounts(t *testing.T) {
	findings := []agent.Finding{
		{ID: "1", Severity: "critical", Title: "c", Body: "b"},
		{ID: "2", Severity: "major", Title: "c", Body: "b"},
		{ID: "3", Severity: "minor", Title: "c", Body: "b"},
		{ID: "4", Severity: "info", Title: "c", Body: "b"},
		{ID: "5", Severity: "info", Title: "c", Body: "b"},
	}
	aggregated := agent.AggregatedResult{Findings: findings}

	report := generateFinalReport(aggregated, nil, nil, 100)

	// 5 findings: 1 critical + 1 major + 1 minor + 2 info
	// rawDeduction = 25 + 12 + 5 + 2*1 = 44, multiplier=1.0 → score = 56
	if report.Score != 56 {
		t.Fatalf("expected score 56 (1*25+1*12+1*5+2*1=44 deducted), got %d", report.Score)
	}

	// verdict: 1 blocker → REQUEST_CHANGES
	if report.Verdict != "REQUEST_CHANGES" {
		t.Fatalf("expected REQUEST_CHANGES (1 blocker), got %s", report.Verdict)
	}

	// buckets: info findings go into suggestions, but score must count them as info
	if len(report.Suggestions) != 3 { // 1 minor + 2 info
		t.Fatalf("expected 3 suggestions, got %d", len(report.Suggestions))
	}
}

// TestGenerateFinalReportInfoNotMinor 验证 info 和 minor 的扣分不同。
// 如果 info 被误计为 minor，分数会比正确值低。
func TestGenerateFinalReportInfoNotMinor(t *testing.T) {
	// 4 info findings: correct score = 100 - round(4*1*1.0) = 96
	infoOnly := agent.AggregatedResult{
		Findings: []agent.Finding{
			{ID: "1", Severity: "info", Title: "i", Body: "b"},
			{ID: "2", Severity: "info", Title: "i", Body: "b"},
			{ID: "3", Severity: "info", Title: "i", Body: "b"},
			{ID: "4", Severity: "info", Title: "i", Body: "b"},
		},
	}
	report := generateFinalReport(infoOnly, nil, nil, 100)
	if report.Score != 96 {
		t.Fatalf("expected 96 (4 info × 1pt = 4 deducted), got %d — info may be counted as minor (5pt)", report.Score)
	}
}

// TestGenerateFinalReportCaseInsensitiveSeverity 验证大小写/空白容错。
// "Critical" 不应掉入 suggestions，而应正确归为 blocker。
func TestGenerateFinalReportCaseInsensitiveSeverity(t *testing.T) {
	aggregated := agent.AggregatedResult{
		Findings: []agent.Finding{
			{ID: "1", Severity: "Critical", Title: "c", Body: "b"}, // 大写
			{ID: "2", Severity: " MAJOR ", Title: "m", Body: "b"},  // 带空白
			{ID: "3", Severity: "", Title: "u", Body: "b"},         // 空值 → major
		},
	}
	report := generateFinalReport(aggregated, nil, nil, 100)

	// 1 critical (blocker) + 2 major (shouldFix)
	if len(report.Blockers) != 1 {
		t.Fatalf("expected 1 blocker (Critical normalized), got %d", len(report.Blockers))
	}
	if len(report.ShouldFix) != 2 { // " MAJOR " + "" → both major
		t.Fatalf("expected 2 shouldFix, got %d", len(report.ShouldFix))
	}
	if len(report.Suggestions) != 0 {
		t.Fatalf("expected 0 suggestions, got %d", len(report.Suggestions))
	}
	// score: round((25 + 2*12) * 1.0) = 49 → value = 100-49 = 51
	if report.Score != 51 {
		t.Fatalf("expected 51 (25+24=49 deducted), got %d", report.Score)
	}
}
